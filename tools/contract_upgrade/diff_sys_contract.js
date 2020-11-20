const path = require("path");
const fs = require("fs");
const process = require("process");
const axios = require("axios");

const printDiff = require("print-diff");

function parseArgs() {
  const contract = process.argv[2];
  if (contract == null) {
    const fileName = __filename.split("/").pop();
    console.log(`
usage: node ${fileName} contract-id
example: node ${fileName} vote_producer
`);
    process.exit(1);
  }
  return contract;
}

function getProjectRoot() {
  const projectRoot = path.join(
    process.env.GOPATH,
    "src/github.com/iost-official/go-iost"
  );
  return projectRoot;
}

function getContractFilePathByName(contract) {
  const contractPath = path.join(
    getProjectRoot(),
    "config/genesis/contract",
    ({ vote: "vote_common", auth: "account" }[contract] || contract) + ".js"
  );
  return contractPath;
}

function getRawLocalContract(contract) {
  const contractPath = getContractFilePathByName(contract);
  const code = fs.readFileSync(contractPath, "utf8");
  const abi = JSON.parse(fs.readFileSync(contractPath + ".abi", "utf8")).abi;
  return { code, abi };
}

function getCompileFunction() {
  const requireFromString = require("require-from-string");
  const projectRoot = getProjectRoot();
  const moduleName = path.join(projectRoot, "vm/v8vm/v8/libjs/inject_gas.js");
  const content = fs.readFileSync(moduleName, "utf8");

  const script = `
const escodegen = require('escodegen');
const esprima = require('esprima');
${content}
`;
  const injectGasFunction = requireFromString(script);
  return injectGasFunction;
}

function getCompiledLocalContract(contract) {
  const { code, abi } = getRawLocalContract(contract);
  const injectGas = getCompileFunction();
  const compiledScript = injectGas(code);
  return { code: compiledScript, abi };
}

function normalizeAbis(abis) {
  function fixAmountLimit(amount) {
    amount.val = amount.value;
    delete amount.value;
    return amount;
  }
  function fixAbi(abi) {
    abi.amountLimit = abi.amount_limit.map(fixAmountLimit);
    delete abi.amount_limit;
    return abi;
  }
  return abis.map(fixAbi);
}

async function getOnchainContract(contract) {
  const onchainContract = (
    await axios.get(`https://api.iost.io/getContract/${contract}.iost/true`)
  ).data;
  return {
    code: onchainContract.code,
    lang: onchainContract.language,
    version: onchainContract.version,
    abi: normalizeAbis(onchainContract.abis)
  };
}

async function dumpAbi(contract) {
  const path = getContractFilePathByName(contract) + ".abi";
  const onchainContract = await getOnchainContract(contract);
  const abi = {
    lang: onchainContract.lang,
    version: onchainContract.version,
    abi: onchainContract.abi
  };
  fs.writeFileSync(path, JSON.stringify(abi, null, 4));
}

async function compareContract(contract) {
  const localContract = getCompiledLocalContract(contract);
  const onchainContract = await getOnchainContract(contract);
  console.log("diff of code:");
  printDiff(onchainContract.code, localContract.code);
  console.log("diff of abi:");
  const toString = j => JSON.stringify(j, null, 4);
  printDiff(toString(onchainContract.abi), toString(localContract.abi));
}

async function main() {
  const contract = parseArgs();
  //await compareContract(contract);
  await dumpAbi(contract);
}

main().catch(console.log);
