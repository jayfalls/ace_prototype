#!/usr/bin/env python3.13

# DEPENDENCIES
## Built-in
import argparse
## Local
from logger import logger


# ARGUMENTS
def _get_arguments() -> dict[str, bool]:
    parser = argparse.ArgumentParser()
    parser.add_argument("-d", "--dev", action="store_true", help="Enable dev mode")
    parser.add_argument("-b", "--build", action="store_true", help="Build the container images")
    return vars(parser.parse_args())


# MAIN
def main():
    arguments: dict[str, bool] = _get_arguments()
    logger.startup("Hello ACE!")
    logger.debug("Dev logs...")
    logger.info("Operations logs...")
    logger.warn("Warning logs...")
    logger.error("Error logs...")
    logger.critical("Critical logs...")

if __name__ == "__main__":
    main()