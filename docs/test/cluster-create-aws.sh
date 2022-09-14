#!/bin/bash
# -----------------------------------------------------------------
# usage
if [ "$#" -lt 1 ]; then 
	echo "./cluster-create.sh <namespace> <clsuter name> <service type>" 
	echo "./cluster-create.sh cb-mcks-ns cluster-01 <multi or single>"
	exit 0; 
fi

source ./conf.env

# ------------------------------------------------------------------------------
# const


# -----------------------------------------------------------------
# parameter

# 1. namespace
if [ "$#" -gt 0 ]; then v_NAMESPACE="$1"; else	v_NAMESPACE="${NAMESPACE}"; fi
if [ "${v_NAMESPACE}" == "" ]; then 
	read -e -p "Namespace ? : " v_NAMESPACE
fi
if [ "${v_NAMESPACE}" == "" ]; then echo "[ERROR] missing <namespace>"; exit -1; fi

# 2. Cluster Name
if [ "$#" -gt 1 ]; then v_CLUSTER_NAME="$2"; else	v_CLUSTER_NAME="${CLUSTER_NAME}"; fi
if [ "${v_CLUSTER_NAME}" == "" ]; then 
	read -e -p "Cluster name  ? : "  v_CLUSTER_NAME
fi
if [ "${v_CLUSTER_NAME}" == "" ]; then echo "[ERROR] missing <cluster name>"; exit -1; fi

# 3. Service Type 
if [ "$#" -gt 2  ]; then v_SERVICE_TYPE="$3"; else	v_SERVICE_TYPE="${SERVICE_TYPE}"; fi
if [ "${v_SERVICE_TYPE}" == ""  ]; then
	read -e -p "Service Type  ? : "  v_SERVICE_TYPE
fi
if [ "${v_SERVICE_TYPE}" == ""  ]; then echo "[ERROR] missing <service type>"; exit -1; fi


c_URL_MCKS_NS="${c_URL_MCKS}/ns/${v_NAMESPACE}"


# ------------------------------------------------------------------------------
# print info.
echo ""
echo "[INFO]"
echo "- Namespace                  is '${v_NAMESPACE}'"
echo "- Cluster name               is '${v_CLUSTER_NAME}'"
echo "- Service type               is '${v_SERVICE_TYPE}'" 

# ------------------------------------------------------------------------------
# Create a cluster
create() {

	if [ "$MCKS_CALL_METHOD" == "REST" ]; then
		v_CLUSTER_CONFIG_REQ=$(cat << EOREQ
			"config": {
				"kubernetes": {
					"networkCni": "flannel",
					"podCidr": "10.244.0.0/16",
					"serviceCidr": "10.96.0.0/12",
					"serviceDnsDomain": "cluster.local"
EOREQ
			);

                if [ "$v_SERVICE_TYPE" != "single" ]; then
                        v_CLUSTER_CONFIG_REQ+=$(cat << EOREQ
				}
			}
EOREQ
			);

		else
			v_CLUSTER_CONFIG_REQ+=$(cat << EOREQ
					,
					"cloudConfig": [
						{
							"key": "Zone",
							"value": "ap-northeast-2"
						}
					] 
				}
			}
EOREQ
			);
		fi

		#echo "v_CLUSTER_CONFIG_REQ: "${v_CLUSTER_CONFIG_REQ}
		#echo ${REQ} | jq
		resp=$(curl -sX POST ${c_URL_MCKS_NS}/clusters?minorversion=1.23 -H "${c_CT}" -d @- <<EOF
		{
			"name": "${v_CLUSTER_NAME}",
			"label": "",
			"installMonAgent": "",
			"description": "",
			"serviceType": "${v_SERVICE_TYPE}",
			${v_CLUSTER_CONFIG_REQ},
			"controlPlane": [
				{
					"connection": "config-aws-ap-northeast-2",
					"count": 1,
					"spec": "t2.medium"
				}
			],
			"worker": [
				{
					"connection": "config-aws-ap-northeast-2",
					"count": 2,
					"spec": "t2.medium"
				}
			]
		}
EOF
		); echo ${resp} | jq

	elif [ "$MCKS_CALL_METHOD" == "GRPC" ]; then

		$APP_ROOT/src/grpc-api/cbadm/cbadm cluster create --config $APP_ROOT/src/grpc-api/cbadm/grpc_conf.yaml -i json -o json -d \
		'{
			"namespace":  "'${v_NAMESPACE}'",
			"ReqInfo": {
					"name": "'${v_CLUSTER_NAME}'",
					"label": "",
					"installMonAgent": "no",                              
					"description": "",
					"config": {
						"kubernetes": {
							"networkCni": "canal",
							"podCidr": "10.244.0.0/16",
							"serviceCidr": "10.96.0.0/12",
							"serviceDnsDomain": "cluster.local"
						}
					},
					"controlPlane": [
						{
							"connection": "config-aws-ap-northeast-1",
							"count": 1,
							"spec": "t2.medium"
						}
					],
					"worker": [
						{
							"connection": "config-gcp-asia-northeast3",
							"count": 1,
							"spec": "n1-standard-2"
						}
					]
				}
		}'	

	else
		echo "[ERROR] missing MCKS_CALL_METHOD"; exit -1;
	fi
	
}


# ------------------------------------------------------------------------------
if [ "$1" != "-h" ]; then 
	echo ""
	echo "------------------------------------------------------------------------------"
	create;
fi
