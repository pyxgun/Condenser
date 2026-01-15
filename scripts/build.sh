#!/bin/bash

BINDIR=./bin
MAINDIR=./cmd/condenser
BINNAME=condenser

go build -o $BINDIR/$BINNAME $MAINDIR