# Program to validate upstreams.
# Author: Shubham Singh<shubham.singh@vuclip.com>
# Purpose remove unwanted upstreams
# `pip install crossplane` is required for this code to work
# Usages python nginx-parser.py <path of nginx configuration>
import crossplane
import re
import sys
import glob
import os

nginx_path = sys.argv[1]
if os.path.exists(nginx_path) is False:
    print("Path does not exists")
    sys.exit(10)

upstream = {}
def main(nginx_path):
    print("Got nginx file {}".format(nginx_path))
    print(os.path.join(nginx_path,'nginx.conf'))
    payload = crossplane.parse(os.path.join(nginx_path,'nginx.conf'))
    # print("Payload is {}".format(payload))
    
    for i in payload['config'][0]['parsed']:
        if 'directive' in i and i['directive'] == "http":
            for block in i['block']:
                if block['directive'] == "upstream":
                    upstream[block['args'][0]] = 0

    for i in payload['config'][0]['parsed']:
        if 'directive' in i and i['directive'] == "http":
                if block['directive'] == "map":
                    for arg in block['block']:
                        if arg['args'][0] in upstream:
                            upstream[arg['args'][0]] += 1
    
    print(upstream)
    for up_st in upstream:
        if findfiles(os.path.join(nginx_path), up_st) > 0:
            upstream[up_st] += 1 
    print(upstream)


def findfiles(path, regex):
    regObj = re.compile(regex)
    res = 0
    for root, dirs, fnames in os.walk(path):
        for fname in fnames:
            # print("File name {}".format(os.path.join(root, fname)))
            # res.append(os.path.join(root, fname))
            if fname.endswith(".conf"):
                res += grep(os.path.join(root, fname), regex)
    return res

def grep(filepath, regex):
    regObj = re.compile(regex)
    res = []
    # print("Got file {}: regex:{}".format(filepath,regex))
    with open(filepath) as f:
        for line in f:
            if regObj.search(line):
               return 1
    return 0


main(nginx_path)

# findfiles('/Users/shubhamsingh/project/common_configurations/nginx_browser2_gce_prod/conf.d',"viu-vuclip-servers")

