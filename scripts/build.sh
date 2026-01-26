#!/bin/bash

BINDIR=./bin
MAINDIR=./cmd/condenser
BINNAME=condenser

swag init -g cmd/condenser/main.go

# condenser
go build -o $BINDIR/$BINNAME $MAINDIR
sudo cp $BINDIR/$BINNAME /usr/bin

HOOKMAINDIR=./cmd/raind-hook
HOOKBINNAME=raind-hook

# hook
go build -o $BINDIR/$HOOKBINNAME $HOOKMAINDIR
sudo cp $BINDIR/$HOOKBINNAME /usr/bin