import * as vscode from 'vscode';
import * as http from 'http';

export function activate(context: vscode.ExtensionContext) {
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
            } else {
                vscode.window.showErrorMessage('Codencer daemon responded with error.');
            }
        }).on('error', (e) => {
            vscode.window.showErrorMessage(`Failed to connect: ${e.message}`);
        });
    });

    let startRunCmd = vscode.commands.registerCommand('codencer.startRun', async () => {
        const runId = await vscode.window.showInputBox({ prompt: "Enter a unique Run ID" });
        if (!runId) return;

        const reqData = JSON.stringify({ id: runId, project_id: "vscode-workspace" });
        const req = http.request({
            hostname: '127.0.0.1', port: 8080, path: '/api/v1/runs', method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(reqData) }
        }, (res) => {
            if (res.statusCode === 201) {
                vscode.window.showInformationMessage(`Run ${runId} started successfully.`);
                provider.refresh();
            } else {
                vscode.window.showErrorMessage(`Failed to start run: Received status ${res.statusCode}`);
            }
        });
        req.on('error', (e) => vscode.window.showErrorMessage(`Failed: ${e.message}`));
        req.write(reqData);
        req.end();
    });

    context.subscriptions.push(refreshCmd, connectCmd, startRunCmd);
}

class CodencerProvider implements vscode.TreeDataProvider<vscode.TreeItem> {
    private _onDidChangeTreeData: vscode.EventEmitter<vscode.TreeItem | undefined | void> = new vscode.EventEmitter<vscode.TreeItem | undefined | void>();
    readonly onDidChangeTreeData: vscode.Event<vscode.TreeItem | undefined | void> = this._onDidChangeTreeData.event;

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    getTreeItem(element: vscode.TreeItem): vscode.TreeItem {
        return element;
    }

    getChildren(element?: vscode.TreeItem): Thenable<vscode.TreeItem[]> {
        if (element) {
            return Promise.resolve([]);
        }

        return new Promise((resolve) => {
            http.get('http://127.0.0.1:8080/api/v1/runs', (res) => {
                let data = '';
                res.on('data', chunk => data += chunk);
                res.on('end', () => {
                    try {
                        const runs = JSON.parse(data);
                        if (!runs || runs.length === 0) {
                            resolve([new vscode.TreeItem("No runs found", vscode.TreeItemCollapsibleState.None)]);
                            return;
                        }
                        const items = runs.map((r: any) => {
                            const item = new vscode.TreeItem(`Run: ${r.ID} [${r.State}]`, vscode.TreeItemCollapsibleState.None);
                            item.description = `Project: ${r.ProjectID}`;
                            return item;
                        });
                        resolve(items);
                    } catch (e) {
                         resolve([new vscode.TreeItem("Error parsing runs", vscode.TreeItemCollapsibleState.None)]);
                    }
                });
            }).on('error', () => {
                resolve([new vscode.TreeItem("Daemon disconnected", vscode.TreeItemCollapsibleState.None)]);
            });
        });
    }
}

export function deactivate() {}

