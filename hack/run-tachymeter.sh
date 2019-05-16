#!/bin/bash

while read t; do
  /run/conversion-webhook-example --name "$t"
done </tmp/tachymeter.test
