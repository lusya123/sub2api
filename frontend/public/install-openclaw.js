#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const cp = require('node:child_process');

const REQUIRED_NODE_VERSION = '22.16.0';

function requiredEnv(name) {
  const value = process.env[name];
  if (!value || !value.trim()) {
    throw new Error(`Missing required environment variable: ${name}`);
  }
  return value.trim();
}

function versionAtLeast(current, required) {
  const currentParts = current.split('.').map((part) => Number(part) || 0);
  const requiredParts = required.split('.').map((part) => Number(part) || 0);
  const length = Math.max(currentParts.length, requiredParts.length);

  for (let i = 0; i < length; i += 1) {
    const currentValue = currentParts[i] || 0;
    const requiredValue = requiredParts[i] || 0;
    if (currentValue > requiredValue) return true;
    if (currentValue < requiredValue) return false;
  }

  return true;
}

function ensureSupportedNode() {
  const current = process.versions?.node || '';
  if (!versionAtLeast(current, REQUIRED_NODE_VERSION)) {
    throw new Error(`OpenClaw requires Node.js ${REQUIRED_NODE_VERSION}+ (current: ${current || 'unknown'})`);
  }
}

function commandExists(command) {
  const checker = process.platform === 'win32' ? 'where' : 'command -v';
  try {
    cp.execSync(`${checker} ${command}`, { stdio: 'ignore', shell: true });
    return true;
  } catch {
    return false;
  }
}

function ensureDir(dirPath) {
  fs.mkdirSync(dirPath, { recursive: true });
}

function readJson(filePath) {
  if (!fs.existsSync(filePath)) return null;
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch {
    return null;
  }
}

function writeJson(filePath, data) {
  ensureDir(path.dirname(filePath));
  fs.writeFileSync(filePath, JSON.stringify(data, null, 2), 'utf8');
}

function backupFile(filePath) {
  if (!fs.existsSync(filePath)) return null;
  const ext = path.extname(filePath);
  const name = path.basename(filePath, ext);
  const backup = path.join(path.dirname(filePath), `${name}.backup-${Date.now()}${ext}`);
  fs.copyFileSync(filePath, backup);
  return backup;
}

function npmCommand() {
  return process.platform === 'win32' ? 'npm.cmd' : 'npm';
}

function installOpenClaw() {
  const npm = npmCommand();
  cp.execSync(`${npm} config set registry https://registry.npmmirror.com`, { stdio: 'inherit' });
  try {
    cp.execSync(`${npm} install -g openclaw@latest --registry=https://registry.npmmirror.com`, { stdio: 'inherit' });
  } catch {
    cp.execSync(`${npm} install -g openclaw@latest`, { stdio: 'inherit' });
  }
}

function mergeConfig(existing, baseUrl, modelKey) {
  const normalized = baseUrl.replace(/\/+$/, '');
  const next = existing && typeof existing === 'object' ? { ...existing } : {};

  const existingModels = next.models && typeof next.models === 'object' ? next.models : {};
  const existingProviders = existingModels.providers && typeof existingModels.providers === 'object'
    ? existingModels.providers
    : {};

  next.models = {
    ...existingModels,
    mode: 'merge',
    providers: {
      ...existingProviders,
      openai: {
        ...(existingProviders.openai || {}),
        baseUrl: `${normalized}/v1`,
        models: Array.isArray(existingProviders.openai?.models) ? existingProviders.openai.models : [],
      },
      anthropic: {
        ...(existingProviders.anthropic || {}),
        baseUrl: normalized,
        models: Array.isArray(existingProviders.anthropic?.models) ? existingProviders.anthropic.models : [],
      },
      google: {
        ...(existingProviders.google || {}),
        baseUrl: `${normalized}/v1beta`,
        models: Array.isArray(existingProviders.google?.models) ? existingProviders.google.models : [],
      },
    },
  };

  const existingAgents = next.agents && typeof next.agents === 'object' ? next.agents : {};
  const existingDefaults = existingAgents.defaults && typeof existingAgents.defaults === 'object'
    ? existingAgents.defaults
    : {};
  const existingModel = existingDefaults.model && typeof existingDefaults.model === 'object'
    ? existingDefaults.model
    : {};
  const existingModelMap = existingDefaults.models && typeof existingDefaults.models === 'object'
    ? existingDefaults.models
    : {};

  next.agents = {
    ...existingAgents,
    defaults: {
      ...existingDefaults,
      model: {
        ...existingModel,
        primary: modelKey,
      },
      models: {
        ...existingModelMap,
        [modelKey]: existingModelMap[modelKey] || {},
      },
    },
  };

  next.meta = {
    ...(next.meta && typeof next.meta === 'object' ? next.meta : {}),
    lastTouchedAt: new Date().toISOString(),
  };

  return next;
}

function mergeAuthProfiles(existing, token) {
  const next = existing && typeof existing === 'object' ? { ...existing } : {};
  const currentProfiles = next.profiles && typeof next.profiles === 'object' ? next.profiles : {};

  next.version = 1;
  next.profiles = {
    ...currentProfiles,
    'openai:default': {
      type: 'api_key',
      provider: 'openai',
      key: token,
    },
    'anthropic:default': {
      type: 'api_key',
      provider: 'anthropic',
      key: token,
    },
    'google:default': {
      type: 'api_key',
      provider: 'google',
      key: token,
    },
  };

  return next;
}

function restartGateway() {
  if (!commandExists('openclaw')) return;
  try {
    cp.execSync('openclaw gateway restart', { stdio: 'inherit' });
  } catch {
    console.log('OpenClaw gateway restart failed. Run "openclaw gateway restart" manually if needed.');
  }
}

function main() {
  const token = requiredEnv('OPENCLAW_TOKEN');
  const baseUrl = requiredEnv('OPENCLAW_BASE_URL');
  const modelKey = requiredEnv('OPENCLAW_MODEL');

  ensureSupportedNode();
  installOpenClaw();

  const stateDir = process.env.OPENCLAW_STATE_DIR || process.env.OPENCLAW_CONFIG_DIR || path.join(os.homedir(), '.openclaw');
  const configPath = process.env.OPENCLAW_CONFIG_PATH || path.join(stateDir, 'openclaw.json');
  const profilesPath = path.join(stateDir, 'agents', 'main', 'agent', 'auth-profiles.json');

  backupFile(configPath);
  backupFile(profilesPath);

  const nextConfig = mergeConfig(readJson(configPath), baseUrl, modelKey);
  const nextProfiles = mergeAuthProfiles(readJson(profilesPath), token);

  writeJson(configPath, nextConfig);
  writeJson(profilesPath, nextProfiles);

  restartGateway();

  console.log('');
  console.log('OpenClaw installed and configured.');
  console.log(`Config: ${configPath}`);
  console.log(`Auth:   ${profilesPath}`);
}

try {
  main();
} catch (error) {
  console.error(error.message);
  process.exit(1);
}
