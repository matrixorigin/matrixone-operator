#!/bin/bash

# reg="-[0]"
# host="${POD_NAME}.${SERVICE_NAME}"


# if [[ $host =~ $reg ]];
# then
#     sed  -i "s/\"JOIN\"/\"\"/g" system_vars_config.toml
# else 
#     sed -i "s/\"JOIN\"/\"http\:\/\/${FIRST_NODE}\:40000\"/g" system_vars_config.toml
# fi

sed -i "s/HOST/${POD_NAME}/g" system_vars_config.toml
sed -i "s/NAME/${POD_NAME}/g" system_vars_config.toml
sed -i "s/PREFIX/${PREFIX}/g" system_vars_config.toml

/usr/local/bin/dumb-init ./mo-server ./system_vars_config.toml