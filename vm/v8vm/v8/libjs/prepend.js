console.log("[JS RUNTIME] prepend start");
// load Block
const blockInfo = JSON.parse(blockchain.blockInfo());
const block = {
   number: blockInfo.number,
   parentHash: blockInfo.parent_hash,
   witness: blockInfo.witness,
   time: blockInfo.time
};

// load tx
const txInfo = JSON.parse(blockchain.txInfo());
const tx = {
   time: txInfo.time,
   hash: txInfo.hash,
   expiration: txInfo.expiration,
   gasLimit: txInfo.gas_limit,
   gasRatio: txInfo.gas_ratio,
   authList: txInfo.auth_list,
   publisher: txInfo.publisher
};

(function(){
    console.log("[JS RUNTIME] get rules start");
    let rules = IOSTBlockchain.instance.rules();
    console.log("[JS RUNTIME] get rules end");
    if (rules.is_fork3_2_0) {
        blockchain.caller = function() {
            return JSON.parse(blockchain.contextInfo()).caller;
        }
        tx.amountLimit = txInfo.amount_limit;
        tx.actions = txInfo.actions;
    }

    IOSTBlockchain = null;
    console.log("[JS RUNTIME] prepend end");
})();