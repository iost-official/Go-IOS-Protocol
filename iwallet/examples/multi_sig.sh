set -eu

TEST_USER_ID="testname"
GROUP="group"
IWALLET_CMD='iwallet --chain_id 1020 -s 127.0.0.1:30002' # single node dev chain
seckey1=3MWryACc5nSxRDJCJLe9Xq2spR1j7d5wYbZ4pZN4SvfgUxhG497DQxo5ahENoCnDkLsc7haSveP1q1zkt26JWhog
pubkey1=2ott3o9CZcaoZCe4nGYo1azEfEpY4W771GBFe133WW1p

function clean_account() {
	$IWALLET_CMD account del $TEST_USER_ID
	$IWALLET_CMD account del manager1
	$IWALLET_CMD account del manager2
	$IWALLET_CMD account del manager3
}

function create_account() {
	$IWALLET_CMD account import $TEST_USER_ID $seckey1
	$IWALLET_CMD --account admin account create $TEST_USER_ID --owner $pubkey1 --active $pubkey1 --initial_balance 20 --initial_ram 20000 --initial_gas_pledge 200
	$IWALLET_CMD --account admin account create manager1 --initial_balance 20 --initial_ram 20000 --initial_gas_pledge 200
	$IWALLET_CMD --account admin account create manager2 --initial_balance 20 --initial_ram 20000 --initial_gas_pledge 200
	$IWALLET_CMD --account admin account create manager3 --initial_balance 20 --initial_ram 20000 --initial_gas_pledge 200
}

function split_account_perm() {
	# First, add three items for the active/owner permission.
	# Then, remove the old keypairs.
	# Finally, since 34 + 34 + 34 > 100(100 is the default permission threshold), only when these three users all sign the tx can the tx be succeesfully sent and run.
	$IWALLET_CMD --account $TEST_USER_ID call --signers ${TEST_USER_ID}@owner \
		auth.iost assignPermission \[\"$TEST_USER_ID\",\"active\",\"manager1@active\",34\] \
		auth.iost assignPermission \[\"$TEST_USER_ID\",\"active\",\"manager2@active\",34\] \
		auth.iost assignPermission \[\"$TEST_USER_ID\",\"active\",\"manager3@active\",34\] \
		auth.iost revokePermission \[\"$TEST_USER_ID\",\"active\",\"$pubkey1\"\] \
		auth.iost assignPermission \[\"$TEST_USER_ID\",\"owner\",\"manager1@active\",34\] \
		auth.iost assignPermission \[\"$TEST_USER_ID\",\"owner\",\"manager2@active\",34\] \
		auth.iost assignPermission \[\"$TEST_USER_ID\",\"owner\",\"manager3@active\",34\] \
		auth.iost revokePermission \[\"$TEST_USER_ID\",\"owner\",\"$pubkey1\"\]
	echo "split permission done"
}

function test_singlesig_fail() {
	# We expected the old keypair is invalid now
	$IWALLET_CMD --account $TEST_USER_ID call token.iost transfer [\"iost\",\"$TEST_USER_ID\",\"admin\",\"10\",\"\"] && (
		echo "should fail, but succeed"
		exit 1
	) || echo "command failed as expected"
}

function test_multisig_usage() {
	delay_seconds=5 # In practice, since you need to send tx file to different persons and collect their signatures, 600(10 minutes) or 1200(20 minutes) may be a better choice for this parameter.
	actionStr="[\"iost\",\"$TEST_USER_ID\",\"admin\",\"10\",\"\"]"
	txStr="--tx_time_delay $delay_seconds token.iost transfer $actionStr"
	txFile="tx.json"
	# First, the operator generates the tx file.
	$IWALLET_CMD call --output $txFile $txStr
	# Then, diffrent users sign the tx file and generate signature files seperately.
	# Following commands are assumed to be run by different persons, on different machines.
	$IWALLET_CMD sign --as_publisher_sign $txFile ~/.iwallet/manager1.json sig2
	$IWALLET_CMD sign --as_publisher_sign $txFile ~/.iwallet/manager2.json sig3
	$IWALLET_CMD sign --as_publisher_sign $txFile ~/.iwallet/manager3.json sig4
	# Finally, the operator send to signed tx to the blockchain.
	$IWALLET_CMD --account $TEST_USER_ID send --signature_files sig2,sig3,sig4 --as_publisher_sign $txFile || (
		echo "should succeed, but fail"
		exit 1
	)
}

#clean_account
create_account
split_account_perm
test_multisig_usage
test_singlesig_fail
echo -e "\nmultisig test succeed!\n"
