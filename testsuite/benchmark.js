import http from 'k6/http';
import { check, sleep } from 'k6';
import { FormData } from 'https://jslib.k6.io/formdata/0.0.2/index.js';

export let options = {
    vus: 10, // number of virtual users
    duration: '30s', // duration of the test
};

export default function () {
    let url = 'http://localhost:8181/registers';

    const fd = new FormData();
    fd.append('name', 'test');
    fd.append('password', 'test');
    fd.append('email', 'test@test.com');
    let params = {
        headers: {
            'Content-Type':'multipart/form-data; boundary=' + fd.boundary ,
        },
    };

    let res = http.post(url, fd.body(), params);
    check(res, {
        'is status 200': (r) => r.status === 200,
    });

    sleep(1);
}