package db

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Credentials holds the credentials for S3 access
type S3Credentials struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // For compatibility with Hetzner and other S3-compatible services
}

// BackupConfig holds all configuration needed for the backup system
type BackupConfig struct {
	// Required parameters
	DB             *sql.DB
	S3Creds        S3Credentials
	BucketName     string
	BackupDelta    time.Duration
	RetentionCount int // Number of backups to retain

	// Optional parameters
	TempDir     string // Directory for temporary files
	BackupLabel string // Label to identify backups from this instance
}

// BackupManager handles SQLite database backups
type BackupManager struct {
	cfg          BackupConfig
	lastChecksum string
	shutdown     chan struct{}
	done         chan struct{}
}

// NewBackupManager creates a new backup manager
func NewBackupManager(cfg BackupConfig) (*BackupManager, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("database connection required")
	}
	if cfg.S3Creds.AccessKeyID == "" || cfg.S3Creds.SecretAccessKey == "" {
		return nil, fmt.Errorf("S3 credentials required")
	}
	if cfg.BucketName == "" {
		return nil, fmt.Errorf("bucket name required")
	}
	if cfg.BackupDelta < time.Minute {
		return nil, fmt.Errorf("backup delta must be at least 1 minute")
	}
	if cfg.RetentionCount < 1 {
		return nil, fmt.Errorf("retention count must be at least 1")
	}

	// Set defaults for optional parameters
	if cfg.TempDir == "" {
		cfg.TempDir = os.TempDir()
	}
	if cfg.BackupLabel == "" {
		cfg.BackupLabel = "backup"
	}

	return &BackupManager{
		cfg:      cfg,
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}, nil
}

// Start begins the backup routine
func (m *BackupManager) Start() {
	go m.run()
}

// Stop gracefully shuts down the backup routine
func (m *BackupManager) Stop() {
	close(m.shutdown)
	<-m.done
}

func (m *BackupManager) run() {
	defer close(m.done)

	ticker := time.NewTicker(m.cfg.BackupDelta)
	defer ticker.Stop()

	for {
		select {
		case <-m.shutdown:
			return
		case <-ticker.C:
			if err := m.performBackup(); err != nil {
				log.Printf("backup failed: %v", err)
			}
			if err := m.cleanOldBackups(); err != nil {
				log.Printf("cleanup failed: %v", err)
			}
		}
	}
}

func (m *BackupManager) performBackup() error {
	// Create temporary file
	tmpFile, err := os.CreateTemp(m.cfg.TempDir, "sqlite-backup-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Perform backup
	if err := m.backupDB(tmpPath); err != nil {
		return fmt.Errorf("failed to backup database: %w", err)
	}

	// Calculate checksum
	checksum, err := m.calculateMD5(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Skip if unchanged
	if checksum == m.lastChecksum {
		return nil
	}

	// Upload to S3
	timestamp := time.Now().UTC().Format("20060102_150405")
	s3Key := fmt.Sprintf("%s_%s_%s.sqlite", m.cfg.BackupLabel, timestamp, checksum)

	if err := m.uploadToS3(tmpPath, s3Key); err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	m.lastChecksum = checksum
	return nil
}

func (m *BackupManager) backupDB(destPath string) error {
	// Open destination database
	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return fmt.Errorf("failed to open backup destination: %w", err)
	}
	defer destDB.Close()

	_, err = m.cfg.DB.Exec(fmt.Sprintf(`VACUUM INTO '%s'`, destPath))
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	return nil
}

func (m *BackupManager) calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *BackupManager) uploadToS3(filePath, s3Key string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create new S3 client for this upload
	client := m.createS3Client()

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(m.cfg.BucketName),
		Key:    aws.String(s3Key),
		Body:   file,
	})

	return err
}

func (m *BackupManager) createS3Client() *s3.Client {
	return s3.New(s3.Options{
		AppID: "keypub-backup/0.0.1",

		Region:       m.cfg.S3Creds.Region,
		BaseEndpoint: aws.String(m.cfg.S3Creds.Endpoint),

		Credentials: credentials.StaticCredentialsProvider{Value: aws.Credentials{
			AccessKeyID:     m.cfg.S3Creds.AccessKeyID,
			SecretAccessKey: m.cfg.S3Creds.SecretAccessKey,
		}},
	})
}

func (m *BackupManager) cleanOldBackups() error {
	// Create new S3 client for cleanup
	client := m.createS3Client()

	// List all objects in bucket
	resp, err := client.ListObjects(context.Background(), &s3.ListObjectsInput{
		Bucket: aws.String(m.cfg.BucketName),
		Prefix: aws.String(m.cfg.BackupLabel + "_"), // Only list our backups
	})
	if err != nil {
		return err
	}

	// Filter and sort backups
	var backups []types.Object
	for _, obj := range resp.Contents {
		// Only process files matching our backup pattern
		if strings.HasPrefix(*obj.Key, m.cfg.BackupLabel+"_") {
			backups = append(backups, obj)
		}
	}

	// Sort backups by LastModified timestamp, newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].LastModified.After(*backups[j].LastModified)
	})

	// Delete all backups beyond the retention count
	if len(backups) > m.cfg.RetentionCount {
		for _, backup := range backups[m.cfg.RetentionCount:] {
			_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(m.cfg.BucketName),
				Key:    backup.Key,
			})
			if err != nil {
				log.Printf("failed to delete old backup %s: %v", *backup.Key, err)
			}
		}
	}

	return nil
}
