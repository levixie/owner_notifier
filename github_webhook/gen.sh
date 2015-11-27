#!/bin/bash

pushd $1
git pull
popd

DATE=`date +%Y-%m-%d`
output=owners_$DATE.txt
ruby ../ownership-anlysis/main.rb -r $1 -e someone@quited.com > $output
ln -f -s $output owners.txt
