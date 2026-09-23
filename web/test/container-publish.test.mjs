/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import test from 'node:test';
import {loopbackTestEnvironment} from './loopback-env.mjs';

test('container publication reserves missing images and checks the existing token grant', async () => {
    let exists = false, mode = 'ok';
    const requests = [];
    const server = createServer(async (request, response) => {
        assert.equal(request.headers.authorization, 'Bearer fixture-token');
        let body = '';
        for await (const chunk of request) body += chunk;
        requests.push([request.method, request.url]);
        response.setHeader('Content-Type', 'application/json');
        if (request.url.startsWith('/v2/token?')) {
            const claims = {
                sub: 'publisher',
                access: [{
                    type: 'repository',
                    name: 'oci/renop',
                    actions: mode === 'denied' ? ['pull'] : ['pull', 'push']
                }]
            };
            response.end(JSON.stringify({token: 'header.' + Buffer.from(JSON.stringify(claims)).toString('base64url') + '.signature'}));
        } else if (request.method === 'POST') {
            assert.deepEqual(JSON.parse(body), {image: 'renop', private: false});
            exists = mode !== 'review';
            response.writeHead(mode === 'review' ? 202 : 201).end('{}');
        } else {
            response.writeHead(exists ? 200 : 404).end('{}');
        }
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const run = () => new Promise((resolve, reject) => {
        const child = spawn('pwsh', ['-NoProfile', '-File', fileURLToPath(new URL('../../.github/scripts/prepare-container-publish.ps1', import.meta.url)),
            '-BaseUrl', `http://127.0.0.1:${server.address().port}`], {
            env: loopbackTestEnvironment({
                GITHUB_OUTPUT: '',
                RENOP_PUBLISH_TOKEN: 'fixture-token',
                HTTP_PROXY: 'http://127.0.0.1:1'
            }), timeout: 30000,
        });
        let output = '';
        child.stdout.on('data', value => {
            output += value;
        });
        child.stderr.on('data', value => {
            output += value;
        });
        child.on('error', reject);
        child.on('close', code => resolve({code, output}));
    });
    try {
        const result = await run();
        assert.equal(result.code, 0, result.output);
        assert.equal(requests.filter(([method]) => method === 'POST').length, 1);
        assert.ok(!result.output.includes('fixture-token'));
        assert.ok(requests.every(([, path]) => !path.startsWith('/api/auth/')));
        requests.length = 0;
        assert.equal((await run()).code, 0);
        assert.ok(requests.every(([method]) => method === 'GET'));
        mode = 'denied';
        assert.notEqual((await run()).code, 0);
        mode = 'review';
        exists = false;
        assert.notEqual((await run()).code, 0);
    } finally {
        await new Promise(resolve => server.close(resolve));
    }
});
