#!/bin/bash

rm test.sqlite*

for (( i=1; $i <= 10; i++ ))
do
	./sqlite-check-concurrency.pl &
done
