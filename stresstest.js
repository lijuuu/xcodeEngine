const http = require('k6/http');
const { check, sleep } = require('k6');

export let options = {
  vus: 10,  // Number of virtual users
  duration: '100s',  // Test duration
};

export default function () {
  let res = http.post('http://localhost:8000/execute', JSON.stringify({
    language: "go",
    code: "package main; import \"fmt\"; func main() { fmt.Println(\"Hello\") }"
  }), { headers: { "Content-Type": "application/json" } });

  check(res, {
    "status is 200": (r) => r.status === 200,
    "execution is fast": (r) => r.timings.duration < 2000,
    "response has expected structure": (r) => r.json().hasOwnProperty('output')
  });

  sleep(1);
}
