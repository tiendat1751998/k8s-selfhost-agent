const fs = require('fs');
const path = require('path');

function walkDir(dir) {
    let results = [];
    const list = fs.readdirSync(dir);
    list.forEach(function(file) {
        file = path.join(dir, file);
        if (fs.statSync(file).isDirectory()) {
            results = results.concat(walkDir(file));
        } else if (file.endsWith('.js')) {
            results.push(file);
        }
    });
    return results;
}

const files = walkDir('D:/project/k8sseflhost/frontend/modules/');
files.forEach(file => {
    const content = fs.readFileSync(file, 'utf8');
    // Check for hardcoded arrays of objects assigned to variables
    const matches = content.match(/=\s*\[\s*\{\s*[\w"']+:/g);
    // Also check for mock functions that return static arrays
    const returnMatches = content.match(/return\s*\[\s*\{\s*[\w"']+:/g);
    
    if ((matches && matches.length > 0) || (returnMatches && returnMatches.length > 0)) {
        console.log('FILE: ' + file.replace(/\\\\/g, '/'));
        let apiCall = false;
        if (content.includes('APIClient') || content.includes('fetch(') || content.includes('api.')) {
            apiCall = true;
        }
        console.log('HAS API CALL: ' + apiCall);
        
        const lines = content.split('\n');
        for (let i = 0; i < lines.length; i++) {
            if (lines[i].match(/=\s*\[\s*\{\s*[\w"']+:/) || lines[i].match(/return\s*\[\s*\{\s*[\w"']+:/)) {
                console.log((i+1) + ': ' + lines[i].trim());
                if (i+1 < lines.length) console.log((i+2) + ': ' + lines[i+1].trim());
                if (i+2 < lines.length) console.log((i+3) + ': ' + lines[i+2].trim());
                break;
            }
        }
        console.log('---');
    }
});
