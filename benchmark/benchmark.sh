#!/bin/bash

wrk -t5 -c200 -d30s --latency -s benchmark/random_ids.lua http://localhost:8080
