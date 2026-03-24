"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const vscode = require("vscode");
const http = require("http");
function activate(context) {
    const provider = new CodencerProvider();
    vscode.window.registerTreeDataProvider('codencerRuns', provider);
    let refreshCmd = vscode.commands.registerCommand('codencer.refresh', () => {
        provider.refresh();
    });
    let connectCmd = vscode.commands.registerCommand('codencer.connect', () => {
        http.get('http://127.0.0.1:8080/health', (res) => {
            if (res.statusCode === 200) {
                vscode.window.showInformationMessage('Successfully connected to Codencer Orchestrator daemon.');
                provider.refresh();
            }
            else {
                vscode.window.showErrorMessage('Codencer daemon responded with error.');
            }
        }).on('error', (e) => {
            vscode.window.showErrorMessage(`Failed to connect: ${e.message}`);
        });
    });
    let startRunCmd = vscode.commands.registerCommand('codencer.startRun', async () => {
        const runId = await vscode.window.showInputBox({ prompt: "Enter a unique Run ID" });
        if (!runId)
            return;
        const reqData = JSON.stringify({ id: runId, project_id: "vscode-workspace" });
        const req = http.request({
            hostname: '127.0.0.1', port: 8080, path: '/api/v1/runs', method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(reqData) }
        }, (res) => {
            if (res.statusCode === 201) {
                vscode.window.showInformationMessage(`Run ${runId} started successfully.`);
                provider.refresh();
            }
            else {
                vscode.window.showErrorMessage(`Failed to start run: Received status ${res.statusCode}`);
            }
        });
        req.on('error', (e) => vscode.window.showErrorMessage(`Failed: ${e.message}`));
        req.write(reqData);
        req.end();
    });
    context.subscriptions.push(refreshCmd, connectCmd, startRunCmd);
}
class BaseTreeItem extends vscode.TreeItem {
    constructor(label, collapsibleState, runId, contextValueObj, parentElement, descriptionInfo) {
        super(label, collapsibleState);
        this.label = label;
        this.collapsibleState = collapsibleState;
        this.runId = runId;
        this.contextValueObj = contextValueObj;
        this.parentElement = parentElement;
        this.descriptionInfo = descriptionInfo;
        if (contextValueObj) {
            this.contextValue = contextValueObj;
        }
        if (descriptionInfo) {
            this.description = descriptionInfo;
        }
    }
}
class CodencerProvider {
    constructor() {
        this._onDidChangeTreeData = new vscode.EventEmitter();
        this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    }
    refresh() {
        this._onDidChangeTreeData.fire();
    }
    getTreeItem(element) {
        return element;
    }
    getChildren(element) {
        if (!element) {
            return this.getRuns();
        }
        else if (element.contextValue === 'run') {
            return Promise.resolve([
                new BaseTreeItem("Steps", vscode.TreeItemCollapsibleState.Expanded, element.runId, 'steps-folder'),
                new BaseTreeItem("Gates", vscode.TreeItemCollapsibleState.Expanded, element.runId, 'gates-folder')
            ]);
        }
        else if (element.contextValue === 'steps-folder' && element.runId) {
            return this.getSteps(element.runId);
        }
        else if (element.contextValue === 'gates-folder' && element.runId) {
            return this.getGates(element.runId);
        }
        return Promise.resolve([]);
    }
    getRuns() {
        return new Promise((resolve) => {
            http.get('http://127.0.0.1:8080/api/v1/runs', (res) => {
                let data = '';
                res.on('data', (chunk) => data += chunk);
                res.on('end', () => {
                    try {
                        const runs = JSON.parse(data);
                        if (!runs || runs.length === 0)
                            return resolve([new BaseTreeItem("No runs found", vscode.TreeItemCollapsibleState.None)]);
                        const items = runs.map((r) => new BaseTreeItem(`Run: ${r.ID} [${r.State}]`, vscode.TreeItemCollapsibleState.Collapsed, r.ID, 'run', undefined, `Project: ${r.ProjectID}`));
                        resolve(items);
                    }
                    catch (e) {
                        resolve([new BaseTreeItem("Error parsing runs", vscode.TreeItemCollapsibleState.None)]);
                    }
                });
            }).on('error', () => resolve([new BaseTreeItem("Daemon disconnected", vscode.TreeItemCollapsibleState.None)]));
        });
    }
    getSteps(runId) {
        return new Promise((resolve) => {
            http.get(`http://127.0.0.1:8080/api/v1/runs/${runId}/steps`, (res) => {
                let data = '';
                res.on('data', (chunk) => data += chunk);
                res.on('end', () => {
                    try {
                        const steps = JSON.parse(data) || [];
                        if (steps.length === 0)
                            resolve([new BaseTreeItem("No steps", vscode.TreeItemCollapsibleState.None)]);
                        resolve(steps.map((s) => new BaseTreeItem(`Step: ${s.Title} [${s.State}]`, vscode.TreeItemCollapsibleState.None, runId, 'step', undefined, `Goal: ${s.Goal}`)));
                    }
                    catch (e) {
                        resolve([new BaseTreeItem("Error parsing steps", vscode.TreeItemCollapsibleState.None)]);
                    }
                });
            }).on('error', () => resolve([new BaseTreeItem("Network Error", vscode.TreeItemCollapsibleState.None)]));
        });
    }
    getGates(runId) {
        return new Promise((resolve) => {
            http.get(`http://127.0.0.1:8080/api/v1/runs/${runId}/gates`, (res) => {
                let data = '';
                res.on('data', (chunk) => data += chunk);
                res.on('end', () => {
                    try {
                        const gates = JSON.parse(data) || [];
                        if (gates.length === 0)
                            resolve([new BaseTreeItem("No gates", vscode.TreeItemCollapsibleState.None)]);
                        resolve(gates.map((g) => new BaseTreeItem(`Gate: [${g.Status}]`, vscode.TreeItemCollapsibleState.None, runId, 'gate', undefined, g.Description)));
                    }
                    catch (e) {
                        resolve([new BaseTreeItem("Error parsing gates", vscode.TreeItemCollapsibleState.None)]);
                    }
                });
            }).on('error', () => resolve([new BaseTreeItem("Network Error", vscode.TreeItemCollapsibleState.None)]));
        });
    }
}
function deactivate() { }
//# sourceMappingURL=extension.js.map