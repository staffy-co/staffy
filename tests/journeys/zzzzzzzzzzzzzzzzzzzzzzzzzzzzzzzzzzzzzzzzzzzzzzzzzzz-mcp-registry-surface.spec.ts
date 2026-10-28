import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const mcpPath = `/v1/tenants/${tenantID}/mcp`;
const protocolVersion = '2026-07-28';
const demoTool = 'auto-parts.demo-fitment';
const demoPin = 'auto-parts.demo-fitment@1.0.0';

// z51 — Story 7.3 (Epic 7): the MCP registry surface journey. The
// scripted MCP client (fetch, no SDK) walks the protocol machine of the
// pinned revision 2026-07-28 over the fixture world: the membership
// ladder BEFORE any protocol material, the discover negotiation, the
// gated tool dictionary (the real registry contracts stay honestly
// excluded — the implemented demo tool of the test contour is the only
// exposed one), the validated call path with the by-name refusals, the
// routing-header coherence, the notification receipt, the method
// refusals, the transport budgets, the rate window, the dependency
// rotation (the dynamic half of the AC-3 gate — the tool disappears),
// the fail-closed output (the leaked internal material never travels)
// and the tenant isolation.

const mcpPost = (api: any, body: string, headers: Record<string, string> = {}) =>
  api.post(mcpPath, {
    data: body,
    headers: {
      'Content-Type': 'application/json',
      'Mcp-Method': headers['Mcp-Method'] ?? 'server/discover',
      'MCP-Protocol-Version': headers['MCP-Protocol-Version'] ?? protocolVersion,
      ...(headers['Mcp-Name'] ? { 'Mcp-Name': headers['Mcp-Name'] } : {}),
      ...Object.fromEntries(Object.entries(headers).filter(([k]) => !['Mcp-Method', 'MCP-Protocol-Version', 'Mcp-Name'].includes(k))),
    },
  });

const envelope = (method: string, extra: string = '', id: string | number = 7) =>
  `{"jsonrpc":"2.0","id":${typeof id === 'string' ? `"${id}"` : id},"method":"${method}"${extra},"_meta":{"protocolVersion":"${protocolVersion}"}}`;

test('mcp-registry-surface: the protocol machine, the gate, the call path and the honest budgets', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. The membership ladder BEFORE any protocol material: the
    // foreign tenant is the honest 404 of the problem contract.
    const foreign = await api.post(`/v1/tenants/${foreignTenantID}/mcp`, {
      data: envelope('server/discover'),
      headers: { 'Content-Type': 'application/json', 'Mcp-Method': 'server/discover', 'MCP-Protocol-Version': protocolVersion },
    });
    expect(foreign.status()).toBe(404);
    expect((await foreign.json()).code).toBe('not_found');
    expect(await foreign.text()).not.toContain('jsonrpc');

    // The member without the scopes: the 403 with the scope words,
    // before a single JSON-RPC byte.
    await api.post('/fixture/operator', { form: { second: 'ops' } });
    const refused = await mcpPost(api, envelope('server/discover'));
    expect(refused.status()).toBe(403);
    const refusal = await refused.text();
    expect(refusal).toContain('integrations или configuration');
    expect(refusal).not.toContain('jsonrpc');
    await api.post('/fixture/operator', { form: {} });

    // 2. The discover negotiation: the exact supported list, the factual
    // capabilities and the server identity.
    const discover = await mcpPost(api, envelope('server/discover'));
    expect(discover.status()).toBe(200);
    const discovered = (await discover.json()).result;
    expect(discovered.protocolVersions).toEqual(['2026-07-28']);
    expect(discovered.capabilities.tools.listChanged).toBe(false);
    expect(discovered.capabilities.resources).toBeUndefined();
    expect(discovered.capabilities.prompts).toBeUndefined();
    expect(discovered.serverInfo).toEqual({ name: 'staffy', version: '0.1.0' });

    // The foreign revision claim: the honest 200 error with the list.
    const foreignVersion = await mcpPost(api, `{"jsonrpc":"2.0","id":9,"method":"server/discover","_meta":{"protocolVersion":"2025-11-25"}}`, { 'MCP-Protocol-Version': '2025-11-25' });
    expect(foreignVersion.status()).toBe(200);
    const versionError = (await foreignVersion.json());
    expect(versionError.error.message).toContain('UnsupportedProtocolVersionError');
    expect(versionError.error.data.supportedVersions).toEqual(['2026-07-28']);
    expect(versionError.error.code).toBe(-32602);

    // 3. The routing-claim coherence: the desyncs are the 400 invalid
    // request with the named reason.
    const desyncedMethod = await mcpPost(api, envelope('server/discover'), { 'Mcp-Method': 'tools/list' });
    expect(desyncedMethod.status()).toBe(400);
    expect((await desyncedMethod.json()).error.message).toContain('Mcp-Method');

    const desyncedVersion = await mcpPost(api, `{"jsonrpc":"2.0","id":8,"method":"server/discover","_meta":{"protocolVersion":"2025-06-18"}}`);
    expect(desyncedVersion.status()).toBe(400);
    expect((await desyncedVersion.json()).error.message).toContain('MCP-Protocol-Version');

    // The batch of the old era and the malformed JSON.
    const batch = await mcpPost(api, `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`);
    expect(batch.status()).toBe(400);
    expect((await batch.json()).error.message).toContain('одно JSON-RPC сообщение');
    const malformed = await mcpPost(api, '{');
    expect(malformed.status()).toBe(400);
    expect((await malformed.json()).error.code).toBe(-32700);

    // The notification receipt: the bare 202 without a body.
    const notification = await mcpPost(api, `{"jsonrpc":"2.0","method":"tools/list","_meta":{"protocolVersion":"${protocolVersion}"}}`, { 'Mcp-Method': 'tools/list' });
    expect(notification.status()).toBe(202);
    expect((await notification.text()).length).toBe(0);

    // The method refusals: no streams, no sessions.
    expect((await api.get(mcpPath)).status()).toBe(405);
    expect((await api.delete(mcpPath)).status()).toBe(405);

    // 4. The gated dictionary: the implemented demo tool is the ONLY
    // exposed one — the real registry contracts stay honestly excluded
    // (the double honesty of 7-2 on the machine surface).
    const list = await mcpPost(api, envelope('tools/list'), { 'Mcp-Method': 'tools/list' });
    expect(list.status()).toBe(200);
    const listed = (await list.json()).result.tools;
    expect(listed.map((tool: any) => tool.name)).toEqual([demoTool]);
    const tool = listed[0];
    expect(tool.title).toBe('Демо-доказательство применимости');
    expect(tool.annotations.readOnlyHint).toBe(true);
    expect(tool.inputSchema.type).toBe('object');
    expect(tool.inputSchema.required).toEqual(['part.oem_number']);
    expect(tool.inputSchema.properties['vehicle.year_of_manufacture'].type).toBe('integer');
    expect(tool.inputSchema.properties['vehicle.year_of_manufacture'].maximum).toBe(2100);
    expect(tool.description).toContain(demoPin);

    // 5. The validated call path: the happy walk answers the typed
    // output with the contract pin in the content.
    const call = await mcpPost(api, `{"jsonrpc":"2.0","id":21,"method":"tools/call","params":{"name":"${demoTool}","arguments":{"part.oem_number":"12345-A","vehicle.year_of_manufacture":2020}},"_meta":{"protocolVersion":"${protocolVersion}"}}`, { 'Mcp-Method': 'tools/call', 'Mcp-Name': demoTool });
    expect(call.status()).toBe(200);
    const called = (await call.json()).result;
    expect(called.isError).toBeUndefined();
    expect(called.structuredContent).toEqual({ 'fitment.link_type': 'original', 'fitment.evidence_age_seconds': 42 });
    expect(called.content[0]).toEqual({ type: 'text', text: demoPin });

    // The by-name refusals of the closed input schema (the model
    // self-corrects — SEP-1303).
    const invalidCall = (argumentsJson: string) =>
      mcpPost(api, `{"jsonrpc":"2.0","id":22,"method":"tools/call","params":{"name":"${demoTool}","arguments":${argumentsJson}},"_meta":{"protocolVersion":"${protocolVersion}"}}`, { 'Mcp-Method': 'tools/call', 'Mcp-Name': demoTool });
    for (const [argumentsJson, word] of [
      ['{"part.oem_number":"x","query.unknown":1}', 'query.unknown'],
      ['{"part.oem_number":7}', 'part.oem_number'],
      ['{"part.oem_number":"x","vehicle.year_of_manufacture":1899}', 'ниже границы'],
      ['{}', 'обязательный аргумент part.oem_number'],
    ] as [string, string][]) {
      const outcome = await invalidCall(argumentsJson);
      expect(outcome.status()).toBe(200);
      const result = (await outcome.json()).result;
      expect(result.isError).toBe(true);
      expect(result.content[0].text).toContain(word);
      expect(result.structuredContent).toBeUndefined();
    }

    // The unknown tool and the Mcp-Name coherence.
    const ghost = await mcpPost(api, `{"jsonrpc":"2.0","id":23,"method":"tools/call","params":{"name":"auto-parts.ghost","arguments":{}},"_meta":{"protocolVersion":"${protocolVersion}"}}`, { 'Mcp-Method': 'tools/call', 'Mcp-Name': 'auto-parts.ghost' });
    expect(ghost.status()).toBe(200);
    expect((await ghost.json()).result.content[0].text).toContain('не существует в реестре');

    const nameless = await mcpPost(api, `{"jsonrpc":"2.0","id":24,"method":"tools/call","params":{"name":"${demoTool}","arguments":{"part.oem_number":"x"}},"_meta":{"protocolVersion":"${protocolVersion}"}}`, { 'Mcp-Method': 'tools/call' });
    expect(nameless.status()).toBe(400);
    expect((await nameless.json()).error.message).toContain('Mcp-Name');

    // 6. The dependency rotation: the stale word closes the demo tool —
    // it disappears from the dictionary and the call refuses with the
    // state word (the dynamic half of the AC-3 gate).
    await api.post('/fixture/mcp-dependency-stale', { form: { enabled: '1' } });
    const rotated = await mcpPost(api, envelope('tools/list'), { 'Mcp-Method': 'tools/list' });
    expect(rotated.status()).toBe(200);
    expect((await rotated.json()).result.tools).toEqual([]);
    const refusedCall = await mcpPost(api, `{"jsonrpc":"2.0","id":25,"method":"tools/call","params":{"name":"${demoTool}","arguments":{"part.oem_number":"x"}},"_meta":{"protocolVersion":"${protocolVersion}"}}`, { 'Mcp-Method': 'tools/call', 'Mcp-Name': demoTool });
    expect(refusedCall.status()).toBe(200);
    const refusedResult = (await refusedCall.json()).result;
    expect(refusedResult.isError).toBe(true);
    expect(refusedResult.content[0].text).toContain('зависимость не жива');
    expect(refusedResult.content[0].text).toContain('stale');
    await api.post('/fixture/mcp-dependency-stale', { form: {} });
    const restored = await mcpPost(api, envelope('tools/list'), { 'Mcp-Method': 'tools/list' });
    expect((await restored.json()).result.tools.map((tool: any) => tool.name)).toEqual([demoTool]);

    // 7. The fail-closed output: the leaked undeclared key refuses the
    // whole answer — the internal material never travels.
    await api.post('/fixture/mcp-adapter-leaky', { form: { enabled: '1' } });
    const leaky = await mcpPost(api, `{"jsonrpc":"2.0","id":26,"method":"tools/call","params":{"name":"${demoTool}","arguments":{"part.oem_number":"x"}},"_meta":{"protocolVersion":"${protocolVersion}"}}`, { 'Mcp-Method': 'tools/call', 'Mcp-Name': demoTool });
    expect(leaky.status()).toBe(200);
    const leakyBody = await leaky.text();
    expect(leakyBody).toContain('незадекларированный ключ');
    expect(leakyBody).not.toContain('внутренний секрет');
    await api.post('/fixture/mcp-adapter-leaky', { form: {} });

    // 8. The declared rate window: the low window refuses the burst with
    // the honest Retry-After.
    await api.post('/fixture/mcp-rate-low', { form: { enabled: '1' } });
    await mcpPost(api, envelope('server/discover'));
    await mcpPost(api, envelope('server/discover'));
    await mcpPost(api, envelope('server/discover'));
    const burst = await mcpPost(api, envelope('server/discover'));
    expect(burst.status()).toBe(429);
    const retryAfter = burst.headers()['retry-after'];
    expect(Number(retryAfter)).toBeGreaterThan(0);
    await api.post('/fixture/mcp-rate-low', { form: {} });
    const afterReset = await mcpPost(api, envelope('server/discover'));
    expect(afterReset.status()).toBe(200);

    // 9. The transport budgets: the content type and the body size.
    const wrongType = await api.post(mcpPath, { data: '{}', headers: { 'Content-Type': 'text/plain', 'Mcp-Method': 'server/discover', 'MCP-Protocol-Version': protocolVersion } });
    expect(wrongType.status()).toBe(415);
    const oversized = await api.post(mcpPath, { data: 'x'.repeat(256 * 1024 + 1), headers: { 'Content-Type': 'application/json', 'Mcp-Method': 'server/discover', 'MCP-Protocol-Version': protocolVersion } });
    expect(oversized.status()).toBe(413);
  } finally {
    await api.post('/fixture/operator', { form: {} });
    await api.post('/fixture/mcp-dependency-stale', { form: {} });
    await api.post('/fixture/mcp-adapter-leaky', { form: {} });
    await api.post('/fixture/mcp-rate-low', { form: {} });
    await page.close();
  }
});
