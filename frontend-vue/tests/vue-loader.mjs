import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { parse, compileScript } from '@vue/compiler-sfc';
import ts from 'typescript';

export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (err) {
    if (specifier.startsWith('.')) {
      try {
        return await nextResolve(specifier + '.ts', context);
      } catch {}
      try {
        return await nextResolve(specifier + '.vue', context);
      } catch {}
    }
    throw err;
  }
}

export async function load(url, context, nextLoad) {
  if (url.endsWith('.css')) {
    return {
      format: 'module',
      shortCircuit: true,
      source: 'export default {};'
    };
  }
  if (url.endsWith('.vue')) {
    const filePath = fileURLToPath(url);
    const source = fs.readFileSync(filePath, 'utf8');
    const { descriptor } = parse(source, { filename: filePath });
    let scriptCode = 'export default {}';
    if (descriptor.script || descriptor.scriptSetup) {
      const compiled = compileScript(descriptor, { id: 'test-sfc', inlineTemplate: true });
      scriptCode = compiled.content;
    }
    const transpiled = ts.transpileModule(scriptCode, {
      compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 }
    }).outputText;
    const cleanJs = transpiled.replace(/import\s+['"][^'"]+\.css['"];?/g, '');
    return {
      format: 'module',
      shortCircuit: true,
      source: cleanJs
    };
  }
  return nextLoad(url, context);
}
