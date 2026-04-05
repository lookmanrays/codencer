"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const vscode = require("vscode");
const http = require("http");
function activate(context) {
    const client = new CodencerClient('127.0.0.1', 8085);
    const provider = new CodencerProvider(client);
    vscode.window.registerTreeDataProvider('codencerRuns', provider);
    // Commands
    context.subscriptions.push(vscode.commands.registerCommand('codencer.refresh', () => provider.refresh()), vscode.commands.registerCommand('codencer.connect', async () => {
        try {
            await client.get('/health');
            vscode.window.showInformationMessage('Connected to Codencer daemon.');
            provider.refresh();
        }
        catch (e) {
            vscode.window.showErrorMessage(`Daemon unreachable: ${e.message}`);
        }
    }), vscode.commands.registerCommand('codencer.approveGate', (item) => handleGateAction(client, item, 'approve', provider)), vscode.commands.registerCommand('codencer.rejectGate', (item) => handleGateAction(client, item, 'reject', provider)), vscode.commands.registerCommand('codencer.retryStep', async (item) => {
        const stepId = item.id;
        if (!stepId)
            return;
        try {
            await client.post(`/api/v1/steps/${stepId}/retry`, {});
            vscode.window.showInformationMessage(`Retrying step ${stepId}...`);
            provider.refresh();
        }
        catch (e) {
            vscode.window.showErrorMessage(`Retry failed: ${e.message}`);
        }
    }), vscode.commands.registerCommand('codencer.inspectResult', (item) => inspectJson(client, `/api/v1/steps/${item.id}/result`, `Result: ${item.id}`)), vscode.commands.registerCommand('codencer.inspectValidations', (item) => inspectJson(client, `/api/v1/steps/${item.id}/validations`, `Validations: ${item.id}`)), vscode.commands.registerCommand('codencer.openArtifact', (filePath) => {
        vscode.workspace.openTextDocument(filePath).then(doc => vscode.window.showTextDocument(doc));
    }));
}
async function handleGateAction(client, item, action, provider) {
    let gateId = item.id;
    if (!gateId)
        return;
    // If it's the "Action Required" item, the id is "gate-{stepId}"
    // The backend expects "gate-{stepId}"
    try {
        await client.post(`/api/v1/gates/${gateId}`, { action });
        vscode.window.showInformationMessage(`Gate ${action}d successfully.`);
        provider.refresh();
    }
    catch (e) {
        vscode.window.showErrorMessage(`Failed to ${action} gate: ${e.message}`);
    }
}
async function inspectJson(client, path, title) {
    try {
        const data = await client.get(path);
        const doc = await vscode.workspace.openTextDocument({
            content: JSON.stringify(data, null, 2),
            language: 'json'
        });
        await vscode.window.showTextDocument(doc);
    }
    catch (e) {
        vscode.window.showErrorMessage(`Inspection failed: ${e.message}`);
    }
}
class CodencerClient {
    constructor(host, port) {
        this.host = host;
        this.port = port;
    }
    get(path) {
        return this.request('GET', path);
    }
    post(path, body) {
        return this.request('POST', path, body);
    }
    request(method, path, body) {
        return new Promise((resolve, reject) => {
            const bodyStr = body ? JSON.stringify(body) : '';
            const options = {
                hostname: this.host,
                port: this.port,
                path,
                method,
                headers: body ? {
                    'Content-Type': 'application/json',
                    'Content-Length': Buffer.byteLength(bodyStr)
                } : {}
            };
            const req = http.request(options, (res) => {
                let data = '';
                res.on('data', chunk => data += chunk);
                res.on('end', () => {
                    if (res.statusCode && res.statusCode >= 200 && res.statusCode < 300) {
                        try {
                            resolve(data ? JSON.parse(data) : {});
                        }
                        catch (e) {
                            resolve(data);
                        }
                    }
                    else {
                        reject(new Error(`Server returned ${res.statusCode}: ${data}`));
                    }
                });
            });
            req.on('error', reject);
            if (bodyStr)
                req.write(bodyStr);
            req.end();
        });
    }
}
class BaseTreeItem extends vscode.TreeItem {
    constructor(label, collapsibleState, id, contextValueObj, parentMetadata) {
        super(label, collapsibleState);
        this.label = label;
        this.collapsibleState = collapsibleState;
        this.id = id;
        this.contextValueObj = contextValueObj;
        this.parentMetadata = parentMetadata;
        this.contextValue = contextValueObj;
    }
}
class CodencerProvider {
    constructor(client) {
        this.client = client;
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    }
    refresh() {
        this._onDidChangeTreeData.fire();
    }
    getTreeItem(element) {
        return element;
    }
    async getChildren(element) {
        if (!element) {
            return this.getRuns();
        }
        switch (element.contextValue) {
            case 'run':
                return this.getSteps(element.id);
            case 'step':
                return this.getStepDetails(element.id, element.parentMetadata);
            default:
                return [];
        }
    }
    async getRuns() {
        try {
            const runs = await this.client.get('/api/v1/runs');
            if (!runs || runs.length === 0)
                return [new BaseTreeItem("No runs found", vscode.TreeItemCollapsibleState.None)];
            return runs.map((r) => {
                const item = new BaseTreeItem(`Run: ${r.id}`, vscode.TreeItemCollapsibleState.Collapsed, r.id, 'run');
                item.description = r.state;
                item.tooltip = `Project: ${r.project_id}\nUpdated: ${r.updated_at}`;
                item.iconPath = this.getRunIcon(r.state);
                return item;
            });
        }
        catch (e) {
            return [new BaseTreeItem("Daemon disconnected", vscode.TreeItemCollapsibleState.None)];
        }
    }
    async getSteps(runId) {
        try {
            const steps = await this.client.get(`/api/v1/runs/${runId}/steps`);
            if (!steps || steps.length === 0)
                return [new BaseTreeItem("No steps", vscode.TreeItemCollapsibleState.None)];
            return steps.map((s) => {
                const item = new BaseTreeItem(s.title, vscode.TreeItemCollapsibleState.Collapsed, s.id, 'step', s);
                item.description = s.state;
                item.tooltip = `Goal: ${s.goal}\nAdapter: ${s.adapter}`;
                item.iconPath = this.getStepIcon(s.state);
                return item;
            });
        }
        catch (e) {
            return [];
        }
    }
    async getStepDetails(stepId, step) {
        const items = [];
        // Add Gate if step is pinned/needs approval
        if (step.State === 'needs_approval') {
            const gateItem = new BaseTreeItem("Action Required: Approve/Reject", vscode.TreeItemCollapsibleState.None, `gate-${stepId}`, 'gate');
            gateItem.iconPath = new vscode.ThemeIcon('question');
            items.push(gateItem);
        }
        // Folders/Lists (Simulated categories for now)
        const resultItem = new BaseTreeItem("Result", vscode.TreeItemCollapsibleState.None, stepId, 'result', step);
        resultItem.command = { title: "Inspect Result", command: "codencer.inspectResult", arguments: [resultItem] };
        resultItem.iconPath = new vscode.ThemeIcon('json');
        items.push(resultItem);
        const validationsItem = new BaseTreeItem("Validations", vscode.TreeItemCollapsibleState.None, stepId, 'validations', step);
        validationsItem.command = { title: "Inspect Validations", command: "codencer.inspectValidations", arguments: [validationsItem] };
        validationsItem.iconPath = new vscode.ThemeIcon('shield');
        items.push(validationsItem);
        try {
            const artifacts = await this.client.get(`/api/v1/steps/${stepId}/artifacts`);
            if (artifacts && artifacts.length > 0) {
                items.push(...artifacts.map((a) => {
                    const artItem = new BaseTreeItem(`${a.type}: ${a.name || a.id}`, vscode.TreeItemCollapsibleState.None, a.id, 'artifact');
                    artItem.description = `${a.size} bytes`;
                    artItem.command = { title: "Open", command: "codencer.openArtifact", arguments: [a.path] };
                    artItem.iconPath = new vscode.ThemeIcon('file');
                    return artItem;
                }));
            }
        }
        catch (e) { }
        return items;
    }
    getRunIcon(state) {
        switch (state) {
            case 'running': return new vscode.ThemeIcon('play', new vscode.ThemeColor('debugIcon.startForeground'));
            case 'completed': return new vscode.ThemeIcon('check', new vscode.ThemeColor('testing.iconPassed'));
            case 'failed': return new vscode.ThemeIcon('error', new vscode.ThemeColor('testing.iconFailed'));
            case 'paused_for_gate': return new vscode.ThemeIcon('pause', new vscode.ThemeColor('debugIcon.pauseForeground'));
            default: return new vscode.ThemeIcon('circle-outline');
        }
    }
    getStepIcon(state) {
        switch (state) {
            case 'running': return new vscode.ThemeIcon('sync~spin');
            case 'completed': return new vscode.ThemeIcon('pass-filled', new vscode.ThemeColor('testing.iconPassed'));
            case 'failed_terminal': return new vscode.ThemeIcon('error', new vscode.ThemeColor('testing.iconFailed'));
            case 'needs_approval': return new vscode.ThemeIcon('warning', new vscode.ThemeColor('testing.iconQueued'));
            default: return new vscode.ThemeIcon('circle-outline');
        }
    }
}
function deactivate() { }
//# sourceMappingURL=extension.js.map