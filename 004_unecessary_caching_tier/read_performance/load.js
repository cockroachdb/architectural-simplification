import http from 'k6/http';
import { check } from 'k6';
import { randomItem } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

const products = JSON.parse(open("./ids.json"));

export const options = {
  scenarios: {
    contacts: {
      duration: '10s',
      executor: 'constant-arrival-rate',
      rate: 2500,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 500,
    },
  },
};

export default function () {
  const product = randomItem(products);
  const res = http.get(`http://localhost:3000/products/${product}/stock`);
  check(res, { 'status was 200': (r) => r.status == 200 });
}