#!/usr/bin/env python3
"""
Make a log file with a long log line -- long enough to crash an early version
of ecslog.

Usage:
    python3 mk-long-log-line.py [MESSAGE-LEN] > crash-long-line.log
"""

import json
import sys

# Add a long message to this.
record = {"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":""}

# Message field length is first arg.
messageLen = 1000
if len(sys.argv) >= 2:
    try:
        messageLen = int(sys.argv[1])
    except ValueError:
        pass

message = '.' * messageLen
for i in range(10, messageLen, 10):
    s = str(i)
    if i + len(s) < messageLen:
        message = message[:i-1] + s + message[i - 1 + len(s):]
# print(message)

record["message"] = message
print(json.dumps(record, separators=(',', ':')))
