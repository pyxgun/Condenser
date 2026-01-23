#!/bin/bash

BINDIR=./bin
MAINDIR=./cmd/condenser
BINNAME=condenser

swag init -g cmd/condenser/main.go
go build -o $BINDIR/$BINNAME $MAINDIR

sudo cp $BINDIR/$BINNAME /usr/bin