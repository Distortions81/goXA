#!/bin/bash

go build  -ldflags="-w" -gcflags="all=-l=4" .