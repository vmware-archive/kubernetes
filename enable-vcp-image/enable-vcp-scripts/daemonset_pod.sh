#!/bin/bash
source $(dirname "$0")/common_func.sh
source $(dirname "$0")/exit_codes.sh

echo "Running script in the Pod:" $POD_NAME "deployed on the Node:" $NODE_NAME

# read secret keys from volume /secret-volume/ and set values in an environment
read_secret_keys
