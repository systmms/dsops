#!/bin/bash
# LocalStack initialization script for dsops integration tests
# This script runs when LocalStack services are ready

set -e

echo "LocalStack initialization script started"

# AWS CLI configuration for LocalStack
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1

# Wait for services to be fully ready
sleep 2

echo "LocalStack services initialized successfully"
