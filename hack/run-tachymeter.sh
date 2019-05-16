#!/bin/bash

while read t; do
  echo "$t"
  /run/conversion-webhook-example --name "$t"
done </tmp/tachymeter.test
