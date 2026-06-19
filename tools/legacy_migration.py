#!/usr/bin/env python3
import os
import logging

# Configure the logger
logger = logging.getLogger(__name__)
logger.setLevel(logging.ERROR)

# Define the backup directory
BACKUP_DIR = 'backup'

try:
    # Check if the backup directory exists
    if not os.path.exists(BACKUP_DIR):
        raise FileNotFoundError(f'The backup directory {BACKUP_DIR} does not exist.')
    # Check if the backup directory is accessible
    if not os.access(BACKUP_DIR, os.R_OK):
        raise OSError(f'The backup directory {BACKUP_DIR} is not accessible.')
    # Perform backup restoration/read logic here
    # ... 
except (OSError, FileNotFoundError) as e:
    # Log an informative error message using the configured logger
    logger.error(f'Error: {e}')
    # Return a graceful exit code when backup operations fail
    exit(1)