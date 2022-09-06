#!/bin/bash

v_CSP="$1"
v_CONTENTS="$2"

if [ "${v_CSP}" == "openstack"  ]; then

sudo tee cloud-config > /dev/null << EOF
[Global]
${v_CONTENTS}
EOF

elif [ "${v_CSP}" == "aws" ]; then

sudo tee cloud-config > /dev/null << EOF
[Global]
${v_CONTENTS}
EOF

fi
