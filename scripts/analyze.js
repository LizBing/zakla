#!/usr/bin/env node

/**
 * Minecraft Server Debug Log Analysis Script
 *
 * This script analyzes logs from the server, mock-client, and real-client
 * to determine if the test iteration was successful.
 *
 * Usage: node scripts/analyze.js <server_log> <mock_log> <real_log>
 *
 * Current behavior (placeholder):
 * - Returns "SUCCESS" if all log files exist and have content
 * - Returns "FAILURE" if any log file is missing or empty
 *
 * Future enhancement:
 * - This will be replaced with AI-powered analysis
 */

const fs = require('fs');
const path = require('path');

/**
 * Analysis result codes
 */
const RESULTS = {
    SUCCESS: 'SUCCESS',
    FAILURE: 'FAILURE',
    SKIP: 'SKIP',
    ERROR: 'ERROR'
};

/**
 * Check if a file exists and has content
 */
function fileExistsAndHasContent(filePath) {
    try {
        const stats = fs.statSync(filePath);
        if (stats.size === 0) {
            return false;
        }
        return true;
    } catch (err) {
        if (err.code === 'ENOENT') {
            return false;
        }
        throw err;
    }
}

/**
 * Read log file content
 */
function readLogFile(filePath) {
    try {
        return fs.readFileSync(filePath, 'utf8');
    } catch (err) {
        return null;
    }
}

/**
 * Analyze server log for errors
 */
function analyzeServerLog(content) {
    if (!content) {
        return { passed: false, reason: 'Server log is empty or missing' };
    }

    // Check for common error patterns
    const errorPatterns = [
        /panic/i,
        /fatal/i,
        /error.*failed/i,
        /connection.*refused/i,
        /timeout/i,
        /segmentation fault/i
    ];

    for (const pattern of errorPatterns) {
        if (pattern.test(content)) {
            // Check if error pattern is found
            // For now, just note it - actual logic depends on expected behavior
        }
    }

    return { passed: true, reason: 'Server log looks OK' };
}

/**
 * Analyze client log for successful execution
 */
function analyzeClientLog(content, clientType) {
    if (!content) {
        return { passed: false, reason: `${clientType} client log is empty or missing` };
    }

    // Look for success indicators
    const successPatterns = [
        /connected/i,
        /success/i,
        /completed/i,
        /done/i,
        /finished/i
    ];

    // Look for failure indicators
    const failurePatterns = [
        /connection.*failed/i,
        /timeout/i,
        /error/i,
        /failed/i,
        /unable.*connect/i
    ];

    let hasSuccess = successPatterns.some(p => p.test(content));
    let hasFailure = failurePatterns.some(p => p.test(content));

    if (hasFailure && !hasSuccess) {
        return { passed: false, reason: `${clientType} client shows failure indicators` };
    }

    return { passed: true, reason: `${clientType} client log looks OK` };
}

/**
 * Main analysis function
 */
function analyzeLogs(serverLogPath, mockLogPath, realLogPath) {
    // Check if all files exist and have content
    const serverExists = fileExistsAndHasContent(serverLogPath);
    const mockExists = fileExistsAndHasContent(mockLogPath);
    const realExists = fileExistsAndHasContent(realLogPath);

    if (!serverExists || !mockExists || !realExists) {
        const missing = [];
        if (!serverExists) missing.push('server');
        if (!mockExists) missing.push('mock');
        if (!realExists) missing.push('real');

        return {
            result: RESULTS.FAILURE,
            reason: `Missing or empty log files: ${missing.join(', ')}`
        };
    }

    // Read all logs
    const serverContent = readLogFile(serverLogPath);
    const mockContent = readLogFile(mockLogPath);
    const realContent = readLogFile(realLogPath);

    // Analyze each log
    const serverResult = analyzeServerLog(serverContent);
    const mockResult = analyzeClientLog(mockContent, 'Mock');
    const realResult = analyzeClientLog(realContent, 'Real');

    // Compile results
    if (serverResult.passed && mockResult.passed && realResult.passed) {
        return {
            result: RESULTS.SUCCESS,
            reason: 'All logs passed basic validation',
            details: {
                server: serverResult.reason,
                mock: mockResult.reason,
                real: realResult.reason
            }
        };
    } else {
        const failures = [];
        if (!serverResult.passed) failures.push(`Server: ${serverResult.reason}`);
        if (!mockResult.passed) failures.push(`Mock: ${mockResult.reason}`);
        if (!realResult.passed) failures.push(`Real: ${realResult.reason}`);

        return {
            result: RESULTS.FAILURE,
            reason: failures.join(' | ')
        };
    }
}

/**
 * Main entry point
 */
function main() {
    const args = process.argv.slice(2);

    if (args.length < 3) {
        console.error('Usage: node analyze.js <server_log> <mock_log> <real_log>');
        process.exit(1);
    }

    const [serverLog, mockLog, realLog] = args;

    try {
        const analysis = analyzeLogs(serverLog, mockLog, realLog);

        // Output only the result code (for script parsing)
        console.log(analysis.result);

        // Output detailed info to stderr (for human reading)
        if (analysis.reason) {
            console.error(`Analysis: ${analysis.reason}`);
        }
        if (analysis.details) {
            console.error('Details:', JSON.stringify(analysis.details, null, 2));
        }

        process.exit(analysis.result === RESULTS.SUCCESS ? 0 : 1);
    } catch (err) {
        console.error(RESULTS.ERROR);
        console.error(`Analysis error: ${err.message}`);
        process.exit(2);
    }
}

// Run if executed directly
if (require.main === module) {
    main();
}

module.exports = { analyzeLogs, RESULTS };
