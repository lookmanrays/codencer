import * as vscode from 'vscode';
import * as http from 'http';

export function activate(context: vscode.ExtensionContext) {
    console.log('Codencer Bridge extension is now active!');

    let connectCmd = vscode.commands.registerCommand('codencer.connect', () => {
        http.get('http://127.0.0.1:8080/health', (res) => {
            if (res.statusCode === 200) {
                vscode.window.showInformationMessage('Successfully connected to Codencer Orchestrator daemon.');
            } else {
                vscode.window.showErrorMessage('Codencer daemon responded with error.');
            }
        }).on('error', (e) => {
            vscode.window.showErrorMessage(`Failed to connect: ${e.message}`);
        });
    });

    let startRunCmd = vscode.commands.registerCommand('codencer.startRun', async () => {
        // Minimal logic to prompt for args and call orchestrator
        const runId = await vscode.window.showInputBox({ prompt: "Enter Run ID" });
        if (!runId) return;

        const reqData = JSON.stringify({
            id: runId,
            project_id: "vscode-workspace"
        });

        const req = http.request({
            hostname: '127.0.0.1',
            port: 8080,
            path: '/api/v1/runs',
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Content-Length': Buffer.byteLength(reqData)
            }
        }, (res) => {
            if (res.statusCode === 201) {
                vscode.window.showInformationMessage(`Run ${runId} started successfully.`);
            } else {
                vscode.window.showErrorMessage(`Failed to start run: Received status ${res.statusCode}`);
            }
        });

        req.on('error', (e) => {
            vscode.window.showErrorMessage(`Failed to connect: ${e.message}`);
        });

        req.write(reqData);
        req.end();
    });

    context.subscriptions.push(connectCmd, startRunCmd);
}

export function deactivate() {}
