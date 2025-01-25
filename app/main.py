#!/usr/bin/env python3.13

# DEPENDENCIES
## Local
from logger import logger

def main():
    logger.startup("Hello ACE!")
    logger.debug("Dev logs...")
    logger.info("Operations logs...")
    logger.warn("Warning logs...")
    logger.error("Error logs...")
    logger.critical("Critical logs...")

main()