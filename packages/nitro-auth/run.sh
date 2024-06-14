#!/bin/sh

if [ ! -d "dist" ]; then
  yarn install
  yarn build
fi

yarn start
