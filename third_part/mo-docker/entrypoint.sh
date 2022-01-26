#!/bin/bash

host="${POD_NAME}.${SERVICE_NAME}"

sed -i "s/HOST/$host/g" system_vars_config.toml
sed -i "s/NAME/${POD_NAME}/g" system_vars_config.toml
sed -i "s/PREFIX/${PREFIX}/g" system_vars_config.toml
sed -i "s/DOMAIN/${SERVICE_NAME}/g" system_vars_config.toml

/usr/local/bin/dumb-init ./mo-server ./system_vars_config.toml