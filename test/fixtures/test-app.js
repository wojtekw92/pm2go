#!/usr/bin/env node

// Test Node.js application for PM2go E2E testing
console.log('Test Node.js app starting...');
console.log('Environment:', process.env.NODE_ENV || 'development');
console.log('Port:', process.env.PORT || 3000);
console.log('PID:', process.pid);

let counter = 0;

// Log every 5 seconds to generate log output for testing
setInterval(() => {
    counter++;
    console.log(`[${new Date().toISOString()}] Heartbeat #${counter} - PID: ${process.pid}`);
    
    // Test environment variables
    if (process.env.TEST_VAR) {
        console.log(`TEST_VAR: ${process.env.TEST_VAR}`);
    }
    
    // Simulate some work
    const memUsage = process.memoryUsage();
    console.log(`Memory: RSS=${Math.round(memUsage.rss/1024/1024)}MB, Heap=${Math.round(memUsage.heapUsed/1024/1024)}MB`);
    
}, 5000);

// Handle graceful shutdown
process.on('SIGTERM', () => {
    console.log('Received SIGTERM, shutting down gracefully...');
    process.exit(0);
});

process.on('SIGINT', () => {
    console.log('Received SIGINT, shutting down gracefully...');
    process.exit(0);
});

// Keep the process running
console.log('Test Node.js app is running. Use Ctrl+C to stop.');
console.log('Environment variables:');
Object.keys(process.env).forEach(key => {
    if (key.startsWith('TEST_') || key.startsWith('NODE_')) {
        console.log(`  ${key}=${process.env[key]}`);
    }
});

// Simulate crash scenario for testing restart functionality
if (process.env.CRASH_AFTER) {
    const crashTime = parseInt(process.env.CRASH_AFTER) * 1000;
    setTimeout(() => {
        console.log('Simulating crash for testing...');
        process.exit(1);
    }, crashTime);
}