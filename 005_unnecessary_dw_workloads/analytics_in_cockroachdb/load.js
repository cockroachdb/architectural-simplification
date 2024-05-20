import http from "k6/http";
import { check, sleep } from "k6";
import { SharedArray } from "k6/data";
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

export const options = {
  vus: 100,
  duration: "1h",
};

const data = new SharedArray("customers", function () {
  // First item is undefined, so data[_VU] starts from 1.
  const items = [undefined];
  for (let i = 0; i < options.vus; i++) {
    items.push(createCustomer(uuidv4()));
  }
  return items;
});

export default function () {
  const headers = {
    "Content-Type": "application/json",
  };

  // Create customer on the first iteration.
  const customer = data[__VU];
  if (__ITER === 0) {
    const headers = {
      "Content-Type": "application/json",
    };
    const res = http.post(
      `http://localhost:3000/customers`,
      JSON.stringify(customer),
      { headers: headers }
    );

    check(res, { "status was 200": (r) => r.status == 200 });
    return
  }

  // Simulate browsing.
  var res = http.get(`http://localhost:3000/products`);
  check(res, { "status was 200": (r) => r.status == 200 });
  sleep(randomIntBetween(0, 3));

  let chosenProducts = JSON.parse(res.body)
    .sort(() => 0.5 - Math.random())
    .slice(0, Math.floor(Math.random() * 5) + 1);

  // Simulate order.
  const order = createOrder(customer.id, chosenProducts);
  res = http.post(`http://localhost:3000/orders`, JSON.stringify(order), {
    headers: headers,
  });

  check(res, { "status was 200": (r) => r.status == 200 });
  sleep(randomIntBetween(0, 3));

  // Simulate payment.
  const payment = createPayment(order);
  res = http.post(`http://localhost:3000/payments`, JSON.stringify(payment), {
    headers: headers,
  });

  check(res, { "status was 200": (r) => r.status == 200 });
  sleep(randomIntBetween(0, 3));
}

function createCustomer(id) {
  return {
    id: id,
    email: `${(Math.random() + 1).toString(36).substring(2)}@gmail.com`,
  };
}

function createOrder(customerID, items) {
  return {
    id: uuidv4(),
    customer_id: customerID,
    items: items,
    total: items.reduce((n, { price }) => parseFloat(n) + parseFloat(price), 0),
  };
}

function createPayment(order) {
  return {
    id: uuidv4(),
    order_id: order.id,
    amount: order.total,
  };
}
