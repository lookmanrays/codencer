import * as vscode from 'vscode';
import * as http from 'http';

export function activate(context: vscode.ExtensionContext) {
    const client = new CodencerClient('127.0.0.1', 8085);
    const provider = new CodencerProvider(client);
    
    vscode.window.registerTreeDataProvider('codencerRuns', provider);

    // Commands
    context.subscriptions.push(
        vscode.commands.registerCommand('codencer.refresh', () => provider.refresh()),
        
        vscode.commands.registerCommand('codencer.connect', async () => {
            try {
                await client.get('/health');
                vscode.window.showInformationMessage('Connected to Codencer daemon.');
                provider.refresh();
            } catch (e: any) {
                vscode.window.showErrorMessage(`Daemon unreachable: ${e.message}`);
            }
        }),

        vscode.commands.registerCommand('codencer.approveGate', (item: BaseTreeItem) => 
            handleGateAction(client, item, 'approve', provider)),
        
        vscode.commands.registerCommand('codencer.rejectGate', (item: BaseTreeItem) => 
            handleGateAction(client, item, 'reject', provider)),

        vscode.commands.registerCommand('codencer.retryStep', async (item: BaseTreeItem) => {
            const stepId = item.id;
            if (!stepId) return;
            try {
                await client.post(`/api/v1/steps/${stepId}/retry`, {});
                vscode.window.showInformationMessage(`Retrying step ${stepId}...`);
                provider.refresh();
            } catch (e: any) {
                vscode.window.showErrorMessage(`Retry failed: ${e.message}`);
            }
        }),

        vscode.commands.registerCommand('codencer.inspectResult', (item: BaseTreeItem) => 
            inspectJson(client, `/api/v1/steps/${item.id}/result`, `Result: ${item.id}`)),

        vscode.commands.registerCommand('codencer.inspectValidations', (item: BaseTreeItem) => 
            inspectJson(client, `/api/v1/steps/${item.id}/validations`, `Validations: ${item.id}`)),

        vscode.commands.registerCommand('codencer.openArtifact', (filePath: string) => {
            vscode.workspace.openTextDocument(filePath).then(doc => vscode.window.showTextDocument(doc));
        })
    );
}

async function handleGateAction(client: CodencerClient, item: BaseTreeItem, action: string, provider: CodencerProvider) {
    let gateId = item.id;
    if (!gateId) return;
    
    // If it's the "Action Required" item, the id is "gate-{stepId}"
    // The backend expects "gate-{stepId}"
    try {
        await client.post(`/api/v1/gates/${gateId}`, { action });
        vscode.window.showInformationMessage(`Gate ${action}d successfully.`);
        provider.refresh();
    } catch (e: any) {
        vscode.window.showErrorMessage(`Failed to ${action} gate: ${e.message}`);
    }
}

async function inspectJson(client: CodencerClient, path: string, title: string) {
    try {
        const data = await client.get(path);
        const doc = await vscode.workspace.openTextDocument({
            content: JSON.stringify(data, null, 2),
            language: 'json'
        });
        await vscode.window.showTextDocument(doc);
    } catch (e: any) {
        vscode.window.showErrorMessage(`Inspection failed: ${e.message}`);
    }
}

class CodencerClient {
    constructor(private readonly host: string, private readonly port: number) {}

    get(path: string): Promise<any> {
        return this.request('GET', path);
    }

    post(path: string, body: any): Promise<any> {
        return this.request('POST', path, body);
    }

    private request(method: string, path: string, body?: any): Promise<any> {
        return new Promise((resolve, reject) => {
            const bodyStr = body ? JSON.stringify(body) : '';
            const options: http.RequestOptions = {
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
                        } catch (e) {
                            resolve(data);
                        }
                    } else {
                        reject(new Error(`Server returned ${res.statusCode}: ${data}`));
                    }
                });
            });

            req.on('error', reject);
            if (bodyStr) req.write(bodyStr);
            req.end();
        });
    }
}

class BaseTreeItem extends vscode.TreeItem {
    constructor(
        public readonly label: string,
        public readonly collapsibleState: vscode.TreeItemCollapsibleState,
        public readonly id?: string,
        public readonly contextValueObj?: string,
        public readonly parentMetadata?: any
    ) {
        super(label, collapsibleState);
        this.contextValue = contextValueObj;
    }
}

class CodencerProvider implements vscode.TreeDataProvider<BaseTreeItem> {
    private _onDidChangeTreeData = new vscode.EventEmitter<BaseTreeItem | undefined | void>();
    readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

    constructor(private readonly client: CodencerClient) {}

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    getTreeItem(element: BaseTreeItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: BaseTreeItem): Promise<BaseTreeItem[]> {
        if (!element) {
            return this.getRuns();
        }

        switch (element.contextValue) {
            case 'run':
                return this.getSteps(element.id!);
            case 'step':
                return this.getStepDetails(element.id!, element.parentMetadata);
            default:
                return [];
        }
    }

    private async getRuns(): Promise<BaseTreeItem[]> {
        try {
            const runs = await this.client.get('/api/v1/runs');
            if (!runs || runs.length === 0) return [new BaseTreeItem("No runs found", vscode.TreeItemCollapsibleState.None)];
            
            return runs.map((r: any) => {
                const item = new BaseTreeItem(`Run: ${r.id}`, vscode.TreeItemCollapsibleState.Collapsed, r.id, 'run');
                item.description = r.state;
                item.tooltip = `Project: ${r.project_id}\nUpdated: ${r.updated_at}`;
                item.iconPath = this.getRunIcon(r.state);
                return item;
            });
        } catch (e) {
            return [new BaseTreeItem("Daemon disconnected", vscode.TreeItemCollapsibleState.None)];
        }
    }

    private async getSteps(runId: string): Promise<BaseTreeItem[]> {
        try {
            const steps = await this.client.get(`/api/v1/runs/${runId}/steps`);
            if (!steps || steps.length === 0) return [new BaseTreeItem("No steps", vscode.TreeItemCollapsibleState.None)];
            
            return steps.map((s: any) => {
                const item = new BaseTreeItem(s.title, vscode.TreeItemCollapsibleState.Collapsed, s.id, 'step', s);
                item.description = s.state;
                item.tooltip = `Goal: ${s.goal}\nAdapter: ${s.adapter}`;
                item.iconPath = this.getStepIcon(s.state);
                return item;
            });
        } catch (e) { return []; }
    }

    private async getStepDetails(stepId: string, step: any): Promise<BaseTreeItem[]> {
        const items: BaseTreeItem[] = [];

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
                items.push(...artifacts.map((a: any) => {
                    const artItem = new BaseTreeItem(`${a.type}: ${a.name || a.id}`, vscode.TreeItemCollapsibleState.None, a.id, 'artifact');
                    artItem.description = `${a.size} bytes`;
                    artItem.command = { title: "Open", command: "codencer.openArtifact", arguments: [a.path] };
                    artItem.iconPath = new vscode.ThemeIcon('file');
                    return artItem;
                }));
            }
        } catch (e) {}

        return items;
    }

    private getRunIcon(state: string) {
        switch (state) {
            case 'running': return new vscode.ThemeIcon('play', new vscode.ThemeColor('debugIcon.startForeground'));
            case 'completed': return new vscode.ThemeIcon('check', new vscode.ThemeColor('testing.iconPassed'));
            case 'failed': return new vscode.ThemeIcon('error', new vscode.ThemeColor('testing.iconFailed'));
            case 'paused_for_gate': return new vscode.ThemeIcon('pause', new vscode.ThemeColor('debugIcon.pauseForeground'));
            default: return new vscode.ThemeIcon('circle-outline');
        }
    }

    private getStepIcon(state: string) {
        switch (state) {
            case 'running': return new vscode.ThemeIcon('sync~spin');
            case 'completed': return new vscode.ThemeIcon('pass-filled', new vscode.ThemeColor('testing.iconPassed'));
            case 'failed_terminal': return new vscode.ThemeIcon('error', new vscode.ThemeColor('testing.iconFailed'));
            case 'needs_approval': return new vscode.ThemeIcon('warning', new vscode.ThemeColor('testing.iconQueued'));
            default: return new vscode.ThemeIcon('circle-outline');
        }
    }
}

export function deactivate() {}

