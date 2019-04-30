#!/usr/bin/env python

from os.path import expanduser
import sys
import yaml

with open(expanduser("~")+"/.kube/config", 'r') as f:
  try:
    config = yaml.safe_load(f)
    for user in config['users']:
      if user['name'] == config['current-context']:
        print(user['user']['token'])
        sys.exit()
    print('user not found')
  except yaml.YAMLError as exc:
    print(exc)
