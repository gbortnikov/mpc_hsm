import { ethers } from "ethers";

/**
 * Генерация неподписанной ETH транзакции для MPC подписи
 */

interface TransactionParams {
  to: string;
  value: string; // в ETH
  nonce: number;
  gasLimit?: bigint;
  maxFeePerGas?: bigint;
  maxPriorityFeePerGas?: bigint;
  chainId?: number;
}

function createUnsignedTransaction(params: TransactionParams): {
  serializedTx: string;
  txHash: string;
  tx: ethers.TransactionLike;
} {
  const tx: ethers.TransactionLike = {
    to: params.to,
    value: ethers.parseEther(params.value),
    nonce: params.nonce,
    gasLimit: params.gasLimit ?? 21000n,
    maxFeePerGas: params.maxFeePerGas ?? ethers.parseUnits("50", "gwei"),
    maxPriorityFeePerGas:
      params.maxPriorityFeePerGas ?? ethers.parseUnits("2", "gwei"),
    chainId: params.chainId ?? 1, // mainnet
    type: 2, // EIP-1559
  };

  // Сериализуем неподписанную транзакцию
  const serializedTx = ethers.Transaction.from(tx).unsignedSerialized;

  // Хеш для подписи (keccak256 от сериализованной транзакции)
  const txHash = ethers.keccak256(serializedTx);

  return { serializedTx, txHash, tx };
}

function generateSigningMessage(txHash: string): {
  messageBytes: Uint8Array;
  messageHex: string;
} {
  // Для ECDSA подписи используется 32-байтный хеш
  const messageBytes = ethers.getBytes(txHash);
  const messageHex = txHash;

  return { messageBytes, messageHex };
}

// Пример использования
async function main() {
  console.log("=== Генерация ETH транзакции для MPC подписи ===\n");

  // Параметры транзакции
  const params: TransactionParams = {
    to: "0xc201ac168Ce1Eb97D8D733F4488e76C7015a7590", // адрес получателя
    value: "0.1", // 0.1 ETH
    nonce: 0, // получить актуальный nonce от провайдера
    gasLimit: 21000n,
    maxFeePerGas: ethers.parseUnits("50", "gwei"),
    maxPriorityFeePerGas: ethers.parseUnits("2", "gwei"),
    chainId: 1, // Ethereum mainnet (5 для Goerli, 11155111 для Sepolia)
  };

  console.log("Параметры транзакции:");
  console.log(`  To: ${params.to}`);
  console.log(`  Value: ${params.value} ETH`);
  console.log(`  Nonce: ${params.nonce}`);
  console.log(`  Gas Limit: ${params.gasLimit}`);
  console.log(`  Chain ID: ${params.chainId}`);
  console.log();

  // Создаём неподписанную транзакцию
  const { serializedTx, txHash } = createUnsignedTransaction(params);

  console.log("Сериализованная транзакция (unsigned):");
  console.log(`  ${serializedTx}`);
  console.log();

  console.log("Хеш транзакции для подписи:");
  console.log(`  ${txHash}`);
  console.log();

  // Генерируем сообщение для подписи
  const { messageBytes, messageHex } = generateSigningMessage(txHash);

  console.log("Сообщение для MPC подписи:");
  console.log(`  Hex: ${messageHex}`);

}

main().catch(console.error);


// вызов из терминала npx tsx generatemsg.ts
