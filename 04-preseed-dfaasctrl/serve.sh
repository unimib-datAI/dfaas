#!/bin/sh

set -e

cd $(dirname "$0")

# Start the PHP server in the current directory
php -S 0.0.0.0:8080 -t .
