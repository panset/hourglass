#!/bin/sh
# SPDX-License-Identifier: MIT
exec perl -e 'alarm shift; exec @ARGV' "$@"
