const fs = require('fs');
const path = require('path');

function walkDir(dir) {
    let results = [];
    const list = fs.readdirSync(dir);
    list.forEach(function(file) {
        file = path.join(dir, file);
        const stat = fs.statSync(file);
        if (stat && stat.isDirectory()) {
            results = results.concat(walkDir(file));
        } else if (file.endsWith('.js')) {
            results.push(file);
        }
    });
    return results;
}

const jsFiles = walkDir('D:/project/k8sseflhost/frontend/');
let mockPatterns = ['mockData', 'testData', 'sampleData', 'prod-us-east', 'demo-cluster', 'static', 'fake'];
let mockFiles = [];

jsFiles.forEach(file => {
    const content = fs.readFileSync(file, 'utf8');
    const lines = content.split('\n');
    let hasMock = false;
    let mockLines = [];
    let hasApiCall = content.includes('apiClient') || content.includes('APIClient') || content.includes('fetch(') || content.includes('api.');
    
    lines.forEach((line, index) => {
        if (mockPatterns.some(p => line.toLowerCase().includes(p.toLowerCase()))) {
            hasMock = true;
            mockLines.push({ line: index + 1, content: line.trim() });
        }
    });

    if (content.includes('id: 1') || content.includes('status: "Running"')) {
        hasMock = true;
    }

    if (hasMock) {
        mockFiles.push({
            file: file.replace('D:\\\\project\\\\k8sseflhost\\\\frontend\\\\', '').replace('D:/project/k8sseflhost/frontend/', ''),
            mockLines: mockLines.slice(0, 5),
            hasApiCall: hasApiCall
        });
    }
});
console.log(JSON.stringify(mockFiles, null, 2));
