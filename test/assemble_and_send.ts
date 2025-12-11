import { ethers } from "ethers";

/**
 * Сборка транзакции из параметров + подписи MPC и отправка
 */

// RPC
const RPC_URL = "https://virtual.mainnet.eu.rpc.tenderly.co/80034cff-1024-4c66-b0ef-ec7a8707b81b";

// ==========================================
// ПАРАМЕТРЫ ТРАНЗАКЦИИ
// ==========================================
const txParams = {
  to: "0xc201ac168Ce1Eb97D8D733F4488e76C7015a7590",
  value: ethers.parseEther("0.1"),
  nonce: 0,
  gasLimit: 21000n,
  maxFeePerGas: ethers.parseUnits("50", "gwei"),
  maxPriorityFeePerGas: ethers.parseUnits("2", "gwei"),
  chainId: 1,
  type: 2, // EIP-1559
};

// ==========================================
// ПОДПИСЬ ОТ MPC
// ==========================================
const mpcSignature = {
  r: "0x06cc8ca8e01294e1d1dcf528f6f5cc715c4d856ff79907dc96d02fcbc89e9fda",
  s: "0x493878e9c6678fc5e077edcc018155d240660f5f1569f389ec60db0e73c6369a",
  v: 27, // v=0 от MPC -> 27 для Ethereum
};

async function main() {
  console.log("=== Сборка и отправка транзакции ===\n");

  // 1. Создаём неподписанную транзакцию
  const unsignedTx = ethers.Transaction.from(txParams);
  const unsignedSerialized = unsignedTx.unsignedSerialized;
  const txHash = ethers.keccak256(unsignedSerialized);

  console.log("Параметры транзакции:");
  console.log(`  To: ${txParams.to}`);
  console.log(`  Value: ${ethers.formatEther(txParams.value)} ETH`);
  console.log(`  Nonce: ${txParams.nonce}`);
  console.log(`  Chain ID: ${txParams.chainId}`);
  console.log(`\nTX Hash (для подписи): ${txHash}`);

  // 2. Собираем подписанную транзакцию
  const signedTx = ethers.Transaction.from({
    ...txParams,
    signature: mpcSignature,
  });

  const signedSerialized = signedTx.serialized;

  console.log(`\n--- Результат сборки ---`);
  console.log(`Подписанная TX: ${signedSerialized}`);

  // 3. Восстанавливаем адрес из подписи
  const recoveredAddress = signedTx.from;
  console.log(`\nАдрес из подписи (MPC кошелёк): ${recoveredAddress}`);

  // 4. Восстанавливаем публичный ключ
  const publicKey = ethers.SigningKey.recoverPublicKey(txHash, mpcSignature);
  console.log(`Публичный ключ: ${publicKey}`);

  // 5. Подключаемся к сети
  console.log(`\n--- Подключение к сети ---`);
  const provider = new ethers.JsonRpcProvider(RPC_URL);

  try {
    const network = await provider.getNetwork();
    console.log(`Сеть: Chain ID ${network.chainId}`);
  } catch (e) {
    console.log(`Ошибка подключения к RPC: ${e}`);
    return;
  }

  // 6. Проверяем баланс
  if (recoveredAddress) {
    const balance = await provider.getBalance(recoveredAddress);
    console.log(`Баланс ${recoveredAddress}: ${ethers.formatEther(balance)} ETH`);

    const needed = txParams.value + (txParams.gasLimit * txParams.maxFeePerGas);
    console.log(`Требуется: ~${ethers.formatEther(needed)} ETH`);

    if (balance < needed) {
      console.log(`\n⚠️  Недостаточно средств для отправки!`);
    }
  }

  // 7. Отправка (раскомментируйте когда готовы)
  
  console.log(`\n--- Отправка транзакции ---`);
  try {
    const txResponse = await provider.broadcastTransaction(signedSerialized);
    console.log(`✅ TX отправлена: ${txResponse.hash}`);

    const receipt = await txResponse.wait();
    console.log(`✅ Подтверждена в блоке: ${receipt?.blockNumber}`);
  } catch (e: any) {
    console.log(`❌ Ошибка: ${e.message}`);
  }
  

  console.log(`\n--- Готово ---`);
  console.log(`Для отправки раскомментируйте блок отправки в коде`);
}

main().catch(console.error);


// вызов из терминала npx tsx assemble_and_send.ts 
