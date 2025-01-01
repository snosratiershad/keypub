package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// ConfigResult holds both the configuration and metadata about how it was loaded
type ConfigResult struct {
	Config *Config
	Source string
}

const usageText = `Usage: keypub [options]

Options:
  -config string
        path to config file (e.g., -config=/path/to/config.json)
  -test
        use test configuration
  -print-config
        print the active configuration and exit
  -help
        display this help message

Examples:
  keypub                           # Run with default production config
  keypub -test                     # Run with built-in test config
  keypub -config=my-config.json    # Run with custom config file
  keypub -print-config            # Print active config and exit
  keypub -test -print-config      # Print test config and exit

Note: -config and -test flags are mutually exclusive`

// LoadFromFlags parses command line flags and loads the appropriate configuration.
// It handles -help, -print-config flags and validates flag combinations.
func LoadFromFlags() (*ConfigResult, error) {
	flags := setupFlags()
	flag.Parse()

	// Check for help flag
	if *flags.help {
		PrintUsage()
		os.Exit(0)
	}

	// Validate flags
	if err := validateFlags(flags); err != nil {
		return nil, err
	}

	result, err := loadConfig(flags)
	if err != nil {
		return nil, err
	}

	// Print config if requested
	if *flags.printConfig {
		result.Print()
		os.Exit(0)
	}

	return result, nil
}

type flags struct {
	configPath  *string
	useTest     *bool
	printConfig *bool
	help        *bool
}

func setupFlags() flags {
	f := flags{
		configPath:  flag.String("config", "", "path to config file"),
		useTest:     flag.Bool("test", false, "use test configuration"),
		printConfig: flag.Bool("print-config", false, "print the active configuration and exit"),
		help:        flag.Bool("help", false, "display help message"),
	}

	// Custom usage function
	flag.Usage = func() {
		PrintUsage()
	}

	return f
}

func validateFlags(flags flags) error {
	if *flags.configPath != "" && *flags.useTest {
		return fmt.Errorf("'-config' and '-test' flags cannot be used together")
	}
	return nil
}

func loadConfig(flags flags) (*ConfigResult, error) {
	var cfg *Config
	var err error
	var source string

	if *flags.useTest {
		cfg = NewTestConfig()
		source = "built-in test configuration"
	} else {
		cfg = NewConfig()
		source = "default production configuration"
	}

	if *flags.configPath != "" {
		err = cfg.LoadConfig(*flags.configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %v", err)
		}
		source = fmt.Sprintf("custom config file: %s", *flags.configPath)
	}

	return &ConfigResult{
		Config: cfg,
		Source: source,
	}, nil
}

// Print prints the configuration in a human-readable format
func (r *ConfigResult) Print() {
	fmt.Printf("Active configuration (source: %s):\n", r.Source)
	jsonBytes, err := json.MarshalIndent(r.Config, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
}

// PrintUsage prints the usage information to stderr
func PrintUsage() {
	fmt.Fprintf(os.Stderr, "%s\n", usageText)
}
