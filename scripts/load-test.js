// Нагрузочный тест: 1000 RPS по одному кошельку в течение 30 секунд.
// Критерий приёмки из ТЗ: ни один запрос не должен вернуть 5xx.
import http from 'k6/http';
import { check } from 'k6';
import { Rate } from 'k6/metrics';

const baseURL = __ENV.BASE_URL || 'http://localhost:8080';
const serverErrors = new Rate('server_errors');

export const options = {
  scenarios: {
    single_wallet_rps: {
      executor: 'constant-arrival-rate',
      rate: 1000,             // 1000 итераций в секунду
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 100,
      maxVUs: 300,
    },
  },
  thresholds: {
    server_errors: ['rate==0'],        // 5xx недопустимы вообще
    http_req_failed: ['rate<0.001'],   // сетевые сбои тоже на нуле
  },
};

// setup: создаём кошелёк с большим балансом, чтобы выводы не упирались в 422.
export function setup() {
  const walletId = '99999999-9999-9999-9999-999999999999';
  const res = http.post(
    `${baseURL}/api/v1/wallet`,
    JSON.stringify({ walletId, operationType: 'DEPOSIT', amount: 100000000 }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(res, { 'setup deposit ok': (r) => r.status === 200 });
  return { walletId };
}

export default function (data) {
  // Половина депозитов, половина выводов, суммы мелкие и симметричные.
  const isDeposit = Math.random() < 0.5;
  const body = JSON.stringify({
    walletId: data.walletId,
    operationType: isDeposit ? 'DEPOSIT' : 'WITHDRAW',
    amount: 1,
  });

  const res = http.post(`${baseURL}/api/v1/wallet`, body, {
    headers: { 'Content-Type': 'application/json' },
  });

  serverErrors.add(res.status >= 500);
  check(res, { 'status 200': (r) => r.status === 200 });
}

// teardown: контрольный баланс после нагрузки.
export function teardown(data) {
  const res = http.get(`${baseURL}/api/v1/wallets/${data.walletId}`);
  console.log(`final balance: ${res.body}`);
}
