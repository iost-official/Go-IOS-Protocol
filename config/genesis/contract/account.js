class Account {
    init() {
    }
    _setAdmin(adminID) {
        storage.put("adminID", adminID);
    }
    initAdmin(adminID) {
        const bn = block.number;
        if(bn !== 0) {
            throw new Error("init out of genesis block")
        }
	this._setAdmin(adminID);
    }
    checkAdminPermission() {
        return blockchain.requireAuth(storage.get("adminID"), "active") || blockchain.requireAuth("admin", "active");
    }
    ensureAdminPermission() {
        if (!this.checkAdminPermission()) {
            throw new Error("require auth failed");
        }
    }
    setAdmin(adminID) {
	this.ensureAdminPermission();
	this._setAdmin(adminID);
    }
    can_update(data) {
	return this.checkAdminPermission();
    }
    _saveAccount(account, payer) {
        if (payer === undefined) {
            payer = account.id
        }
        storage.mapPut("auth", account.id, JSON.stringify(account), payer);
    }

    _loadAccount(id) {
        let a = storage.mapGet("auth", id);
        return JSON.parse(a);
    }

    static _find(items, name) {
        for (let i = 0; i < items.length; i++) {
            if (items[i].id === name) {
                return i
            }
        }
        return -1
    }

    static _findPermission(items, name) {
        const len = name.indexOf("@");
        if (len < 0) {
            for (let i = 0; i < items.length; i++) {
                if (items[i].id === name && items[i].permission === undefined) {
                    return i
                }
            }
        } else if (len > 0) {
            for (let i = 0; i < items.length; i++) {
                if (items[i].id === name.substring(0, len) && items[i].permission === name.substring(len+1, name.length)) {
                    return i
                }
            }
        } else {
            throw "unexpected item"
        }
        return -1
    }

    _hasAccount(id) {
        return storage.mapHas("auth", id);
    }

    _ra(id) {
        if (!blockchain.requireAuth(id, "owner")) {
            throw new Error("require auth failed");
        }
    }

    _checkIdValid(id) {
        if (block.number === 0) {
            return
        }
        if (id.length < 5 || id.length > 11) {
            throw new Error("id invalid. id length should be between 5,11 > " + id)
        }
        if (id.startsWith("Contract")) {
            throw new Error("id invalid. id shouldn't start with 'Contract'.");
        }
        for (let i in id) {
            let ch = id[i];
            if (!(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch === '_')) {
                throw new Error("id invalid. id contains invalid character > " + ch);
            }
        }
    }

    _checkPermValid(perm) {
        if (block.number === 0) {
            return
        }
        if (perm.length < 1 || perm.length > 32) {
            throw new Error("id invalid. id length should be between 1,32 > " + id)
        }
        for (let i in perm) {
            let ch = perm[i];
            if (!(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch === '_')) {
                throw new Error("id invalid. id contains invalid character > " + ch);
            }
        }
    }

    _checkWeight(weight) {
        if (weight <= 0) {
            throw "weight less than zero"
        }
    }

    /**
     * @param  {string} id - this is a string
     *
     */
    signUp(id, owner, active) {
        if (this._hasAccount(id)) {
            throw new Error("id existed > " + id);
        }
        this._checkIdValid(id);
        const referrer = blockchain.publisher();
        let account = {};
        account.id = id;
        account.referrer = referrer;
        account.permissions = {};
        account.permissions.active = {
            name: "active",
            groups: [],
            items: [{
                id: active,
                is_key_pair: true,
                weight: 100,
            }],
            threshold: 100,
        };
        account.permissions.owner = {
            name: "owner",
            groups: [],
            items: [{
                id: owner,
                is_key_pair: true,
                weight: 100,
            }],
            threshold: 100,
        };
        account.groups = {};
        this._saveAccount(account, blockchain.contractName());
        if (block.number !== 0) {
            const defaultGasPledge = "10";
            blockchain.callWithAuth("gas.iost", "pledge", [referrer, id, defaultGasPledge]);
        }

        blockchain.receipt(JSON.stringify([id, owner, active]));
    }

    addPermission(id, perm, thres) {
        this._ra(id);
        this._checkPermValid(perm);
        let acc = this._loadAccount(id);
        if (acc.permissions[perm] !== undefined) {
            throw new Error("permission already exist");
        }
        acc.permissions[perm] = {
            name: perm,
            groups: [],
            items: [],
            threshold: thres,
        };
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, perm, thres]));
    }

    dropPermission(id, perm) {
        this._ra(id);
        if (perm === "active" || perm === "owner") {
            throw "drop active or owner is forbidden"
        }
        let acc = this._loadAccount(id);
        acc.permissions[perm] = undefined;
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, perm]));
    }

    assignPermission(id, perm, un, weight) {
        this._ra(id);
        this._checkWeight(weight);
        let acc = this._loadAccount(id);
        const index = Account._find(acc.permissions[perm].items, un);
        if (index < 0) {
            const len = un.indexOf("@");
            if (len < 0) {
                acc.permissions[perm].items.push({
                    id: un,
                    is_key_pair: true,
                    weight: weight
                });
            } else if (len > 0) {
                acc.permissions[perm].items.push({
                    id: un.substring(0, len),
                    permission: un.substring(len+1, un.length),
                    is_key_pair: false,
                    weight: weight
                });
            } else {
                throw "unexpected item"
            }
        } else {
            acc.permissions[perm].items[index].weight = weight
        }
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, perm, un, weight]));
    }

    revokePermission(id, perm, un) {
        this._ra(id);
        let acc = this._loadAccount(id);
        const index = Account._findPermission(acc.permissions[perm].items, un);
        if (index < 0) {
            throw new Error("item not found");
        } else {
            acc.permissions[perm].items.splice(index, 1)
        }
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, perm, un]));
    }

    addGroup(id, grp) {
        this._ra(id);
        this._checkPermValid(grp);
        let acc = this._loadAccount(id);
        if (acc.groups[grp] !== undefined) {
            throw new Error("group already exist");
        }
        acc.groups[grp] = {
            name: grp,
            items: [],
        };
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, grp]));
    }

    dropGroup(id, group) {
        this._ra(id);
        let acc = this._loadAccount(id);
        acc.groups[group] = undefined;
        for (let i = 0; i < acc.permissions.length; i++) {
            for (let j = 0; j < acc.permissions[i].groups.length; j++) {
                if (acc.permissions[i].groups[j] === group) {
                    acc.permissions[i].groups.splice(j, 1)
                }
            }
        }
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, group]));
    }

    assignGroup(id, group, un, weight) {
        this._ra(id);
        this._checkWeight(weight);
        let acc = this._loadAccount(id);
        const index = Account._find(acc.groups[group].items, un);
        if (index < 0) {
            let len = un.indexOf("@");
            if (len < 0) {
                acc.groups[group].items.push({
                    id: un,
                    is_key_pair: true,
                    weight: weight
                });
            } else {
                acc.groups[group].items.push({
                    id: un.substring(0, len),
                    permission: un.substring(len+1, un.length),
                    is_key_pair: false,
                    weight: weight
                });
            }
        } else {
            acc.groups[group].items[index].weight = weight
        }

        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, group, un, weight]));
    }

    revokeGroup(id, grp, un) {
        this._ra(id);
        let acc = this._loadAccount(id);
        const index = Account._findPermission(acc.groups[grp].items, un);
        if (index < 0) {
            throw new Error("item not found");
        } else {
            acc.groups[grp].items.splice(index, 1)
        }
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, grp, un]));
    }

    assignPermissionToGroup(id, perm, group) {
        this._ra(id);
        let acc = this._loadAccount(id);
        if (acc.groups[group] === undefined) {
            throw new Error("group does not exist");
        }
        acc.permissions[perm].groups.push(group);
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, perm, group]));
    }

    revokePermissionInGroup(id, perm, group) {
        this._ra(id);
        let acc = this._loadAccount(id);
        let index = acc.permissions[perm].groups.indexOf(group);
        if (index > -1) {
            acc.permissions[perm].groups.splice(index, 1);
        }
        this._saveAccount(acc);

        blockchain.receipt(JSON.stringify([id, perm, group]));
    }
}

module.exports = Account;
