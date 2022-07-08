#!/bin/bash

v_CSP="$1"

if [ "${v_CSP}" == "openstack"  ]; then

	v_OPENSTACK_ENDPOINT="${OS_AUTH_URL}"
	if [ "${v_OPENSTACK_ENDPOINT}" == "" ]; then
		read -e -p "Identity Endpoint ? [예:http://123.456.789.123/identity] : "  v_OPENSTACK_ENDPOINT
		if [ "${v_OPENSTACK_ENDPOINT}" == "" ]; then echo "[ERROR] missing <openstack identity endpoint>"; exit -1;fi
	fi

	v_OPENSTACK_USERNAME="${OS_USERNAME}"
	if [ "${v_OPENSTACK_USERNAME}" == "" ]; then
		read -e -p "Username ? [예:admin] : "  v_OPENSTACK_USERNAME
		if [ "${v_OPENSTACK_USERNAME}" == "" ]; then echo "[ERROR] missing <openstack username>"; exit -1;fi
	fi

	v_OPENSTACK_PASSWORD="${OS_PASSWORD}"
	if [ "${v_OPENSTACK_PASSWORD}" == "" ]; then
		read -e -p "Password ? [예:passw0rd] : "  v_OPENSTACK_PASSWORD
		if [ "${v_OPENSTACK_PASSWORD}" == "" ]; then echo "[ERROR] missing <openstack password>"; exit -1;fi
	fi

	v_OPENSTACK_DOMAINNAME="${OS_USER_DOMAIN_NAME}"
	if [ "${v_OPENSTACK_DOMAINNAME}" == "" ]; then
		read -e -p "DomainName ? [예:default] : "  v_OPENSTACK_DOMAINNAME
		if [ "${v_OPENSTACK_DOMAINNAME}" == "" ]; then echo "[ERROR] missing <openstack domainname>"; exit -1;fi
	fi

	v_OPENSTACK_PROJECTID="${OS_PROJECT_ID}"
	if [ "${v_OPENSTACK_PROJECTID}" == "" ]; then
		read -e -p "ProjectID? [예:default] : "  v_OPENSTACK_PROJECTID
		if [ "${v_OPENSTACK_PROJECTID}" == "" ]; then echo "[ERROR] missing <openstack projectid>"; exit -1;fi
	fi

	# region
	v_REGION="${OS_REGION}"
	if [ "${v_REGION}" == "" ]; then
		read -e -p "region ? [예:RegionOne] : "  v_REGION
		if [ "${v_REGION}" == "" ]; then echo "[ERROR] missing region"; exit -1;fi
	fi

	# zone
	v_ZONE="${OS_ZONE}"
	if [ "${v_ZONE}" == "" ]; then
		read -e -p "zone ? [예:RegionOne-1] : "  v_ZONE
		if [ "${v_ZONE}" == "" ]; then v_ZONE="${v_REGION}a";fi
	fi

sudo tee /etc/kubernetes/cloud-config > /dev/null << EOF
[Global]
auth-url="$v_OPENSTACK_ENDPOINT"
username="$v_OPENSTACK_USERNAME"
password="$v_OPENSTACK_PASSWORD"
region="$v_REGION"
domain-name="$v_OPENSTACK_DOMAINNAME"
tenant-id="$v_OPENSTACK_PROJECTID"
EOF


elif [ "${v_CSP}" == "aws" ]; then

    exit 1;

fi
