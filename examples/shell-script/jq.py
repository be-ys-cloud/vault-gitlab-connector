#!/usr/bin/env python2
import fileinput
import json
import sys

field = "/"
if len(sys.argv) >= 2:
    field = sys.argv[1]
data = ""
for line in fileinput.input("-"):
    data += line
j = json.loads(data)
for f in field.split("/"):
    if len(f) > 0:
        j = j[f]
    else:
        j = j
print(j)
