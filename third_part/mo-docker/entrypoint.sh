#!/bin/bash


sed -i "s/HOST/${POD_NAME}/g" system_vars_config.toml
sed -i "s/NAME/${POD_NAME}/g" system_vars_config.toml
sed -i "s/PREFIX/${PREFIX}/g" system_vars_config.toml

/usr/local/bin/dumb-init ./mo-server ./system_vars_config.toml