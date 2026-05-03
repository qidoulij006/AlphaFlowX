#!/bin/bash
#
# AlphaFlowX bootstrap installer
# Served from https://alphaflowx.com/install.sh
#
# This wrapper keeps the public install entrypoint branded while
# delegating to the current maintained code repository installer.
#

set -e

curl -fsSL https://raw.githubusercontent.com/NoFxAiOS/nofx/main/install.sh | bash -s -- "$@"
