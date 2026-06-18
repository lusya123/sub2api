import { defineConfig, loadEnv, Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import checker from 'vite-plugin-checker'
import { resolve } from 'path'

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function injectPublicSettings(backendUrl: string): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        const visitorScript = visitorCookieBootstrapScript()
        try {
          const response = await fetch(`${backendUrl}/api/v1/settings/public`, {
            signal: AbortSignal.timeout(2000)
          })
          if (response.ok) {
            const data = await response.json()
            if (data.code === 0 && data.data) {
              const injectedData = { ...data.data }
              delete injectedData.customer_service_qrcode
              const script = `<script>window.__APP_CONFIG__=${JSON.stringify(injectedData)};</script>${visitorScript}`
              return html.replace('</head>', `${script}\n</head>`)
            }
          }
        } catch (e) {
          console.warn('[vite] 无法获取公开配置，将回退到 API 调用:', (e as Error).message)
        }
        return html.replace('</head>', `${visitorScript}\n</head>`)
      }
    }
  }
}

function visitorCookieBootstrapScript(): string {
  return `<script>
(function(){
  const cookieName = '_xdt_v=';
  const fingerprintCookieName = '_xdt_fp';
  function hasVisitorCookie(){ return document.cookie.indexOf(cookieName) !== -1; }
  function setFingerprintCookie(fingerprint){
    const secure = location.protocol === 'https:' ? '; Secure' : '';
    document.cookie = fingerprintCookieName + '=' + encodeURIComponent(fingerprint) + '; path=/; max-age=1800; SameSite=Lax' + secure;
  }
  async function sha256Hex(value){
    const encoded = new TextEncoder().encode(value);
    const hashBuffer = await window.crypto.subtle.digest('SHA-256', encoded);
    return Array.from(new Uint8Array(hashBuffer)).map(function(b){return b.toString(16).padStart(2,'0');}).join('');
  }
  async function canvasFingerprint(){
    try {
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      if (!ctx) return 'no-canvas';
      ctx.textBaseline = 'top';
      ctx.font = '14px Arial';
      ctx.fillText('fingerprint', 2, 2);
      return canvas.toDataURL().slice(-50);
    } catch(e) { return 'no-canvas'; }
  }
  function webglFingerprint(){
    try {
      const canvas = document.createElement('canvas');
      const gl = canvas.getContext('webgl');
      if (!gl) return 'no-webgl';
      const ext = gl.getExtension('WEBGL_debug_renderer_info');
      if (!ext) return 'webgl';
      return String(gl.getParameter(ext.UNMASKED_RENDERER_WEBGL) || 'webgl').slice(0, 30);
    } catch(e) { return 'no-webgl'; }
  }
  async function collectFingerprint(){
    const data = {
      ua: navigator.userAgent,
      lang: navigator.language,
      tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
      screen: screen.width + 'x' + screen.height,
      color: screen.colorDepth,
      hw: navigator.hardwareConcurrency || 0,
      canvas: await canvasFingerprint(),
      webgl: webglFingerprint()
    };
    return sha256Hex(JSON.stringify(data));
  }
  async function issueVisitorCookie(recover){
    if (!window.crypto || !window.crypto.subtle) return;
    const fingerprint = await collectFingerprint();
    setFingerprintCookie(fingerprint);
    const challengeResp = await fetch('/api/public/visitor/challenge' + (recover ? '?recover=1' : ''), {method:'POST', credentials:'same-origin'});
    if (!challengeResp.ok) return;
    const challengeData = await challengeResp.json();
    const prefix = '0'.repeat(challengeData.difficulty || 4);
    let nonce = 0;
    for (;;) {
      const hash = await sha256Hex(challengeData.challenge + nonce);
      if (hash.startsWith(prefix)) break;
      nonce++;
    }
    await fetch('/api/public/visitor/issue-cookie' + (recover ? '?recover=1' : ''), {
      method:'POST',
      credentials:'same-origin',
      headers:{'Content-Type':'application/json'},
      body:JSON.stringify({challenge:challengeData.challenge, nonce:String(nonce), fingerprint:fingerprint})
    });
  }
  window.__xdtReissueVisitorCookie = function(){ return issueVisitorCookie(true); };
  if (!hasVisitorCookie()) issueVisitorCookie(false).catch(function(e){ console.error('Visitor cookie issue failed', e); });
  const originalFetch = window.fetch;
  if (originalFetch && !window.__xdtVisitorFetchWrapped) {
    window.__xdtVisitorFetchWrapped = true;
    window.fetch = async function(){
      const response = await originalFetch.apply(this, arguments);
      if (response && response.status === 403) {
        response.clone().json().then(async function(data){
          if (data && data.error === 'cookie reputation too low' && window.confirm('您的访问被暂时限制，点击确定完成验证恢复访问。')) {
            document.cookie = '_xdt_v=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
            document.cookie = fingerprintCookieName + '=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
            await issueVisitorCookie(true);
            window.location.reload();
          }
        }).catch(function(){});
      }
      return response;
    };
  }
})();
</script>`
}

export default defineConfig(({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const backendUrl = env.VITE_DEV_PROXY_TARGET || 'http://localhost:8080'
  const devPort = Number(env.VITE_DEV_PORT || 3000)

  return {
    plugins: [
      vue(),
      checker({
        vueTsc: true
      }),
      injectPublicSettings(backendUrl)
    ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
    }
  },
  define: {
    // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
    // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
    __INTLIFY_JIT_COMPILATION__: true
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        /**
         * 手动分包配置
         * 分离第三方库并按功能合并应用代码，避免循环依赖
         */
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Vue 核心库
            if (
              id.includes('/vue/') ||
              id.includes('/vue-router/') ||
              id.includes('/pinia/') ||
              id.includes('/@vue/')
            ) {
              return 'vendor-vue'
            }

            // UI 工具库（较大，单独分离）
            if (id.includes('/@vueuse/') || id.includes('/xlsx/')) {
              return 'vendor-ui'
            }

            // 图表库
            if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
              return 'vendor-chart'
            }

            // 国际化
            if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
              return 'vendor-i18n'
            }

            // 其他小型第三方库合并
            return 'vendor-misc'
          }

          // 应用代码：按入口点自动分包，不手动干预
          // 这样可以避免循环依赖，同时保持合理的 chunk 数量
        }
      }
    }
  },
    server: {
      host: '0.0.0.0',
      port: devPort,
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})
