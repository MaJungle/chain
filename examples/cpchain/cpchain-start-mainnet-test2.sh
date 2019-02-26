#!/usr/bin/env bash

cd "$(dirname "${BASH_SOURCE[0]}")"

#set log level by add parameter:--verbosity 4
# or spec env like this:env CPC_VERBOSITY=4  ./cpchain-start-mainnet-test.sh
#PanicLevel	0
#FatalLevel	1
#ErrorLevel	2
#WarnLevel	3
#InfoLevel	4
#DebugLevel	5


set -u
set -e

validatorip=""
if [ ! $1 ]; then
    validator_ip='127.0.0.1'
else
    validator_ip=$1
fi


export bootnodes="enode://d9bd60488c269f1324ed7811341f4c81ce42ed0531852ea265148a6a7f4cb99d58c95a979de059c5052b1d38eb4462b775f8d8db40f92cf3828d884265176cde@${validator_ip}:30301,\
enode://775cedbf2026c065b67fc80a38995c2999d5b3c1f3a80695115b2606ad4025dacae6947034c89032453c0a71d8b49465f167e9c3b0e85a8cf9b4281e1cc198e4@${validator_ip}:30302,\
enode://b21a1567059168e3cecc7d5c60217cd73dc5b299c3865f2f9eea621a93e7e8bb266f7ade0b1db28160686c740be6de4c1000ef449d7cd8c97388eca1790bc61a@${validator_ip}:30303,\
enode://fb56640fb3b8dec3473ea3906ef59b97c4f7956d86be27ed65908fa706d2fbe91800b7a221ba45127cbe4b5eb26291f7fbd3984cdeab587d6fb53535ce4e0069@${validator_ip}:30304"

export validators="enode://2ddfb534019e6b446fb4465742f266d04fae661089e3dac6a4c841ad0fcf5569f8d049203079bb64e20d1a32fc84b584920839a2120cd5e8886744719452d936@${validator_ip}:30317,\
enode://f2a460e5d5008d0ba8ec0744c90df9d1fc01553e00025af483995a15d89e773de18a37972c38bdcf47917fc820738455b85675bb51b026a75768c68d5540d053@${validator_ip}:30318,\
enode://f3045792b9e9ad894cb36b488f1cf97065013cda9ef60f2b14840214683b3ef3dadf61450a9f677457c1a4b75a5b456947f48f3cb0019c7470cced9af1829993@${validator_ip}:30319,\
enode://be14fce25a846bd5c91728fec7fb7071c98e2b9f8f4b710dbce79d8b6098592591ebeebfe6c59ee5bfd6f75387926f9342ae004d6ff8dcf97fc6d7e91e8f41be@${validator_ip}:30320,\
enode://00e5229f3792264032a335759671996da3714f90f8d19defd0abce4e27515e7e644a76ae19b994f9b28b4d652826fa0766298b60db6df70aa2def7461c50d662@${validator_ip}:30321,\
enode://369699f91013336e4ecf349aac4a4a6ee3957c7c7577996f9db821013e2e232ef8151e200cc2ab7ea9265121642b05b1cd21640d29e1e4bf8f6af737f353275c@${validator_ip}:30322,\
enode://ee4c7418336745ed8a54da5fd8b151ade53b0b2a53b8e1d5eecfae483d15f5ff9e440155c47311dc826c44d44dce0080a6246204ed992f1e37d7094df4289169@${validator_ip}:30323"

export args="run --validators "${validators}" --networkid 0 --bootnodes ${bootnodes} --rpcapi personal,eth,cpc,admission,net,web3,db,txpool,miner --linenumber --runmode mainnet"


echo "bootnodes:${bootnodes}"
echo "validators:${validators}"
echo "args:${args}"

./cpchain-start-mainnet-init.sh

./cpchain-start-mainnet-proposer-2.sh

./cpchain-start-mainnet-proposer-3.sh

./cpchain-start-mainnet-proposer-4.sh

./cpchain-start-mainnet-bank.sh

./cpchain-start-mainnet-civilian.sh

