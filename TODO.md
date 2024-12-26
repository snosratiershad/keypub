# TODO

## High Priority

### Identity Verification Systems
* Add GitHub verification
  * OAuth integration
  * Public key verification via gists
  * Profile linking
* Add Twitter verification
  * OAuth integration
  * Tweet-based verification
  * Handle linking
* Add LinkedIn verification
  * OAuth integration
  * Profile verification
  * Professional identity linking
* Add Reddit verification
  * OAuth integration
  * Comment/post verification mechanism
  * Username linking
* Add HackerNews verification
  * API integration
  * Profile verification
  * Username linking

### Verifiable Log System
* Implement Merkle tree for all operations
  * Hash chain for all registrations
  * Merkle tree for verification lookups
  * Proof generation system
* Create verifiable log
  * Append-only log structure
  * Timestamp server integration
  * Public audit capability
* Add verification tools
  * CLI for log verification
  * API endpoints for proof verification
  * Documentation for verification process

### Configuration Management
* Move hardcoded constants to configuration system
  * Database paths
  * Server ports
  * Rate limiting parameters
  * Email settings
  * File paths
* Support both config file and environment variables
* Add configuration validation
* Add documentation for all configuration options

### Testing
* Add unit tests for critical components:
  * Email validation
  * Rate limiting algorithm
  * Permission system
  * Database operations
* Add integration tests for:
  * Email sending flow
  * Registration process
  * Permission management
* Set up CI pipeline for automated testing

### Logging and Monitoring
* Implement structured logging
* Add request/response logging
* Add error logging with proper context
* Add metrics for:
  * Request rates
  * Error rates
  * Registration success/failure rates
  * Email sending success/failure rates
* Add health check endpoints

## Medium Priority

### Database Improvements
* Implement database migrations system
* Add database connection pooling configuration
* Add database backup system
* Add cleanup routines for old data
* Document database schema

### Code Quality
* Fix error handling in JSON marshaling operations
* Add godoc style comments for all exported functions
* Implement graceful shutdown handling
* Add request context timeout handling
* Clean up duplicate constants

### Documentation
* Add architectural overview document
* Document rate limiting algorithm in detail
* Add deployment guide
* Add troubleshooting guide
* Add API documentation
* Add development setup guide

## Low Priority

### Developer Experience
* Add Makefile for common operations
* Add development environment setup script
* Add example configuration files
* Add contribution guidelines
* Add issue templates

### Security Enhancements
* Add key rotation mechanism
* Add audit logging
* Add rate limit bypassing for allowlisted IPs
* Add automated security scanning in CI

### Features
* Add prometheus metrics endpoint
* Add admin interface for system monitoring
* Add bulk operations support
* Add API versioning
* Support for different email providers

## Notes
- Items are roughly ordered by priority within each section
- Some items may be dependent on others
- Consider creating GitHub issues for tracking these items
- Identity verification systems should be implemented sequentially, not in parallel
- Merkle tree implementation should happen before adding additional verification methods
