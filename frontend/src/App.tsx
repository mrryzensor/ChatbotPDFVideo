import React, { useState, useEffect } from 'react';
import { 
  Bot, 
  FileText, 
  Video, 
  Settings, 
  QrCode, 
  RefreshCw, 
  Send, 
  CheckCircle2, 
  XCircle, 
  Sparkles, 
  Upload, 
  Download, 
  Layers, 
  Scissors, 
  Minimize2, 
  Key, 
  Plus, 
  Trash2, 
  Zap,
  Terminal,
  Play
} from 'lucide-react';

interface AppConfig {
  gemini_api_keys: string[];
  system_prompt: string;
  auto_reply: boolean;
  preferred_model: string;
  yt_dlp_path: string;
  ffmpeg_path: string;
}

interface ChatMessage {
  id: string;
  from: string;
  fromName: string;
  text: string;
  isBot: boolean;
  timestamp: string;
  modelUsed?: string;
  keyUsed?: string;
}

interface DownloadJob {
  id: string;
  url: string;
  format: string;
  quality: string;
  status: string;
  progress: string;
  filename: string;
  filepath: string;
  error?: string;
}

export function App() {
  const [activeTab, setActiveTab] = useState<'bot' | 'pdf' | 'video' | 'settings'>('bot');
  const [config, setConfig] = useState<AppConfig>({
    gemini_api_keys: [],
    system_prompt: '',
    auto_reply: true,
    preferred_model: 'auto',
    yt_dlp_path: 'yt-dlp',
    ffmpeg_path: 'ffmpeg',
  });

  // Bot State
  const [waStatus, setWaStatus] = useState<string>('disconnected');
  const [waQR, setWaQR] = useState<string>('');
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [manualRecipient, setManualRecipient] = useState('');
  const [manualMessage, setManualMessage] = useState('');
  const [cheapestModel, setCheapestModel] = useState<string>('Buscando...');
  const [testPrompt, setTestPrompt] = useState('Hola Gemini, ¿cómo estás?');
  const [testResponse, setTestResponse] = useState<any>(null);
  const [testingGemini, setTestingGemini] = useState(false);

  // PDF State
  const [pdfAction, setPdfAction] = useState<'merge' | 'split' | 'compress'>('merge');
  const [pdfFiles, setPdfFiles] = useState<FileList | null>(null);
  const [pdfSingleFile, setPdfSingleFile] = useState<File | null>(null);
  const [splitSpan, setSplitSpan] = useState<number>(1);
  const [pdfLoading, setPdfLoading] = useState(false);
  const [pdfResult, setPdfResult] = useState<any>(null);

  // Video State
  const [videoUrl, setVideoUrl] = useState('');
  const [videoInfo, setVideoInfo] = useState<any>(null);
  const [fetchingInfo, setFetchingInfo] = useState(false);
  const [downloadFormat, setDownloadFormat] = useState<'video' | 'audio'>('video');
  const [downloadQuality, setDownloadQuality] = useState('1080p');
  const [jobs, setJobs] = useState<DownloadJob[]>([]);

  // Settings State
  const [newKey, setNewKey] = useState('');
  const [saveSuccess, setSaveSuccess] = useState(false);

  // Load config & initial status
  useEffect(() => {
    fetchConfig();
    fetchWhatsAppStatus();
    fetchCheapestModel();
    fetchJobs();

    // Setup polling / websocket
    const interval = setInterval(() => {
      fetchWhatsAppStatus();
      fetchJobs();
    }, 3000);

    return () => clearInterval(interval);
  }, []);

  const fetchConfig = async () => {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const data = await res.json();
        setConfig(data);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const fetchCheapestModel = async () => {
    try {
      const res = await fetch('/api/gemini/models');
      if (res.ok) {
        const data = await res.json();
        setCheapestModel(data.cheapest_model || 'gemini-3.5-flash-lite');
      }
    } catch {
      setCheapestModel('gemini-3.5-flash-lite');
    }
  };

  const fetchWhatsAppStatus = async () => {
    try {
      const res = await fetch('/api/whatsapp/status');
      if (res.ok) {
        const data = await res.json();
        setWaStatus(data.status);
        setWaQR(data.qr_code || '');
      }
      const msgRes = await fetch('/api/whatsapp/messages');
      if (msgRes.ok) {
        const m = await msgRes.json();
        setMessages(m);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const fetchJobs = async () => {
    try {
      const res = await fetch('/api/video/jobs');
      if (res.ok) {
        const data = await res.json();
        setJobs(data);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleSaveConfig = async () => {
    try {
      const res = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      if (res.ok) {
        setSaveSuccess(true);
        setTimeout(() => setSaveSuccess(false), 3000);
        fetchCheapestModel();
      }
    } catch (e) {
      alert('Error guardando configuración');
    }
  };

  const handleAddKey = async () => {
    if (!newKey.trim()) return;
    const updatedKeys = [...config.gemini_api_keys, newKey.trim()];
    const updatedConfig = {
      ...config,
      gemini_api_keys: updatedKeys,
    };
    setConfig(updatedConfig);
    setNewKey('');

    // Guardar automáticamente para persistir inmediatamente la nueva API key en el servidor
    try {
      const res = await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updatedConfig),
      });
      if (res.ok) {
        setSaveSuccess(true);
        setTimeout(() => setSaveSuccess(false), 3000);
        fetchCheapestModel();
      }
    } catch (e) {
      console.error("Error al auto-guardar llave", e);
    }
  };

  const handleRemoveKey = async (idx: number) => {
    const updatedKeys = config.gemini_api_keys.filter((_, i) => i !== idx);
    const updatedConfig = { ...config, gemini_api_keys: updatedKeys };
    setConfig(updatedConfig);

    try {
      await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updatedConfig),
      });
      fetchCheapestModel();
    } catch (e) {
      console.error(e);
    }
  };

  const testGeminiFailover = async () => {
    setTestingGemini(true);
    setTestResponse(null);
    try {
      // Si el usuario tiene una llave escrita en el input pero aún no le dio a agregar, avisarle o usar la configuración actual
      if (config.gemini_api_keys.length === 0 && !newKey.trim()) {
        setTestResponse({
          error: "Por favor agrega al menos una API Key de Google AI Studio en la lista de arriba antes de probar."
        });
        setTestingGemini(false);
        return;
      }

      const res = await fetch('/api/gemini/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt: testPrompt }),
      });
      const data = await res.json();
      setTestResponse(data);
    } catch (e: any) {
      setTestResponse({ error: e.message || 'Error en la petición' });
    } finally {
      setTestingGemini(false);
    }
  };

  const handleConnectWhatsApp = async () => {
    await fetch('/api/whatsapp/connect');
    fetchWhatsAppStatus();
  };

  const handleDisconnectWhatsApp = async () => {
    await fetch('/api/whatsapp/disconnect');
    fetchWhatsAppStatus();
  };

  const handleLogoutWhatsApp = async () => {
    if (confirm('¿Cerrar sesión y desvincular WhatsApp? Deberás escanear el QR nuevamente.')) {
      await fetch('/api/whatsapp/logout');
      fetchWhatsAppStatus();
    }
  };

  const handleSendManual = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!manualRecipient || !manualMessage) return;
    try {
      const res = await fetch('/api/whatsapp/send', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ recipient: manualRecipient, message: manualMessage }),
      });
      if (res.ok) {
        setManualMessage('');
        fetchWhatsAppStatus();
      } else {
        const err = await res.text();
        alert('Error: ' + err);
      }
    } catch (e: any) {
      alert(e.message);
    }
  };

  // PDF Handlers
  const handlePdfProcess = async () => {
    setPdfLoading(true);
    setPdfResult(null);
    const formData = new FormData();

    try {
      if (pdfAction === 'merge') {
        if (!pdfFiles || pdfFiles.length < 2) {
          alert('Por favor selecciona 2 o más archivos PDF para unir');
          setPdfLoading(false);
          return;
        }
        for (let i = 0; i < pdfFiles.length; i++) {
          formData.append('files', pdfFiles[i]);
        }
        const res = await fetch('/api/pdf/merge', { method: 'POST', body: formData });
        const data = await res.json();
        setPdfResult({ type: 'merge', data });
      } else if (pdfAction === 'split') {
        if (!pdfSingleFile) {
          alert('Selecciona un archivo PDF para dividir');
          setPdfLoading(false);
          return;
        }
        formData.append('file', pdfSingleFile);
        formData.append('span', splitSpan.toString());
        const res = await fetch('/api/pdf/split', { method: 'POST', body: formData });
        const data = await res.json();
        setPdfResult({ type: 'split', data });
      } else if (pdfAction === 'compress') {
        if (!pdfSingleFile) {
          alert('Selecciona un archivo PDF para optimizar/comprimir');
          setPdfLoading(false);
          return;
        }
        formData.append('file', pdfSingleFile);
        const res = await fetch('/api/pdf/compress', { method: 'POST', body: formData });
        const data = await res.json();
        setPdfResult({ type: 'compress', data });
      }
    } catch (e: any) {
      alert('Error en el proceso PDF: ' + e.message);
    } finally {
      setPdfLoading(false);
    }
  };

  // Video Handlers
  // Normalize video link on paste or change
  const handleVideoUrlChange = (val: string) => {
    let clean = val.trim();
    // Normalize youtu.be/xxx?si=yyy -> youtube.com/watch?v=xxx
    try {
      if (clean.includes('youtu.be/')) {
        const id = clean.split('youtu.be/')[1].split('?')[0].split('&')[0];
        if (id) clean = `https://www.youtube.com/watch?v=${id}`;
      } else if (clean.includes('/shorts/')) {
        const id = clean.split('/shorts/')[1].split('?')[0].split('&')[0];
        if (id) clean = `https://www.youtube.com/watch?v=${id}`;
      } else if (clean.includes('youtube.com/watch')) {
        // Strip tracking params like si, pp, t
        const urlObj = new URL(clean.startsWith('http') ? clean : `https://${clean}`);
        const v = urlObj.searchParams.get('v');
        if (v) clean = `https://www.youtube.com/watch?v=${v}`;
      }
    } catch {}
    setVideoUrl(clean);
  };

  const handleFetchVideoInfo = async () => {
    if (!videoUrl) return;
    setFetchingInfo(true);
    setVideoInfo(null);
    try {
      const res = await fetch(`/api/video/info?url=${encodeURIComponent(videoUrl.trim())}`);
      if (res.ok) {
        const data = await res.json();
        setVideoInfo(data);
      } else {
        const err = await res.text();
        alert('Error: ' + err);
      }
    } catch (e: any) {
      alert(e.message);
    } finally {
      setFetchingInfo(false);
    }
  };

  const handleStartDownload = async () => {
    if (!videoUrl) return;
    try {
      const res = await fetch('/api/video/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: videoUrl,
          format: downloadFormat,
          quality: downloadQuality,
        }),
      });
      if (res.ok) {
        fetchJobs();
      }
    } catch (e: any) {
      alert(e.message);
    }
  };

  return (
    <div className="flex h-screen bg-slate-950 text-slate-100 overflow-hidden font-sans">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-900 border-r border-slate-800 flex flex-col justify-between">
        <div>
          <div className="p-5 border-b border-slate-800 flex items-center space-x-3">
            <div className="p-2 bg-gradient-to-tr from-indigo-600 to-violet-500 rounded-xl shadow-lg shadow-indigo-500/20">
              <Bot className="w-6 h-6 text-white" />
            </div>
            <div>
              <h1 className="font-bold text-lg leading-tight bg-gradient-to-r from-white to-slate-400 bg-clip-text text-transparent">
                OmniSaaS Hub
              </h1>
              <p className="text-xs text-indigo-400 font-medium">IA & Tools Suite</p>
            </div>
          </div>

          <nav className="p-4 space-y-1.5">
            <button
              onClick={() => setActiveTab('bot')}
              className={`w-full flex items-center space-x-3 px-3.5 py-2.5 rounded-lg text-sm font-medium transition-all ${
                activeTab === 'bot'
                  ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/30 shadow-sm'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
              }`}
            >
              <Bot className="w-4 h-4" />
              <span>Chatbot WhatsApp IA</span>
            </button>

            <button
              onClick={() => setActiveTab('pdf')}
              className={`w-full flex items-center space-x-3 px-3.5 py-2.5 rounded-lg text-sm font-medium transition-all ${
                activeTab === 'pdf'
                  ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/30 shadow-sm'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
              }`}
            >
              <FileText className="w-4 h-4" />
              <span>Gestor de PDFs</span>
            </button>

            <button
              onClick={() => setActiveTab('video')}
              className={`w-full flex items-center space-x-3 px-3.5 py-2.5 rounded-lg text-sm font-medium transition-all ${
                activeTab === 'video'
                  ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/30 shadow-sm'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
              }`}
            >
              <Video className="w-4 h-4" />
              <span>Descargador YT & Web</span>
            </button>

            <button
              onClick={() => setActiveTab('settings')}
              className={`w-full flex items-center space-x-3 px-3.5 py-2.5 rounded-lg text-sm font-medium transition-all ${
                activeTab === 'settings'
                  ? 'bg-indigo-600/15 text-indigo-400 border border-indigo-500/30 shadow-sm'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
              }`}
            >
              <Settings className="w-4 h-4" />
              <span>Configuración & API Keys</span>
            </button>
          </nav>
        </div>

        {/* Status card in sidebar */}
        <div className="p-4 m-3 bg-slate-800/50 rounded-xl border border-slate-800/80">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs text-slate-400">WhatsApp Status</span>
            <span className="flex h-2 w-2 relative">
              <span className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${
                waStatus === 'connected' ? 'bg-emerald-400' : 'bg-amber-400'
              }`} />
              <span className={`relative inline-flex rounded-full h-2 w-2 ${
                waStatus === 'connected' ? 'bg-emerald-500' : 'bg-amber-500'
              }`} />
            </span>
          </div>
          <div className="text-xs font-semibold uppercase tracking-wider text-slate-200">
            {waStatus === 'connected' ? 'En Línea' : waStatus === 'qr_ready' ? 'Esperando Escaneo' : 'Desconectado'}
          </div>
          <div className="mt-2 text-[11px] text-slate-400 truncate flex items-center gap-1">
            <Zap className="w-3 h-3 text-amber-400" />
            <span>Modelo: {cheapestModel}</span>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col overflow-hidden bg-slate-950">
        {/* Header */}
        <header className="h-16 border-b border-slate-800 px-8 flex items-center justify-between bg-slate-900/40 backdrop-blur-md">
          <div className="flex items-center gap-3">
            <h2 className="font-semibold text-lg text-slate-100">
              {activeTab === 'bot' && 'Asistente WhatsApp con Google AI Studio & WhatsMeow'}
              {activeTab === 'pdf' && 'Suite Avanzada de Procesamiento PDF (Merge, Split, Compress)'}
              {activeTab === 'video' && 'Descargador Multimedia Universal con yt-dlp'}
              {activeTab === 'settings' && 'Gestión de Google AI Studio Failover y Sistema'}
            </h2>
          </div>
          <div className="flex items-center gap-3">
            <div className="px-3 py-1 bg-slate-800/80 border border-slate-700/60 rounded-full text-xs font-medium text-slate-300 flex items-center gap-1.5">
              <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
              <span>Failover Activo: {config.gemini_api_keys.length} Llaves</span>
            </div>
          </div>
        </header>

        {/* Scrollable Body */}
        <div className="flex-1 overflow-y-auto p-8">
          {/* BOT TAB */}
          {activeTab === 'bot' && (
            <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 max-w-7xl mx-auto">
              {/* WhatsApp Connection Box */}
              <div className="lg:col-span-4 space-y-6">
                <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl">
                  <div className="flex items-center justify-between pb-4 border-b border-slate-800">
                    <h3 className="font-semibold text-slate-100 flex items-center gap-2">
                      <QrCode className="w-5 h-5 text-indigo-400" />
                      Vinculación WhatsApp
                    </h3>
                    <button
                      onClick={fetchWhatsAppStatus}
                      className="p-1.5 text-slate-400 hover:text-slate-200 hover:bg-slate-800 rounded-lg transition"
                      title="Refrescar estado"
                    >
                      <RefreshCw className="w-4 h-4" />
                    </button>
                  </div>

                  <div className="mt-5 flex flex-col items-center">
                    {waStatus === 'qr_ready' && waQR ? (
                      <div className="text-center space-y-3">
                        <div className="p-3 bg-white rounded-xl shadow-lg inline-block">
                          <img src={waQR} alt="QR de WhatsApp" className="w-56 h-56" />
                        </div>
                        <p className="text-xs text-slate-400 max-w-xs">
                          Abre WhatsApp en tu teléfono, ve a <b>Dispositivos vinculados</b> y escanea este código.
                        </p>
                      </div>
                    ) : waStatus === 'connected' ? (
                      <div className="py-8 text-center space-y-3">
                        <div className="w-16 h-16 bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 rounded-full flex items-center justify-center mx-auto shadow-lg shadow-emerald-500/10">
                          <CheckCircle2 className="w-8 h-8" />
                        </div>
                        <h4 className="font-medium text-slate-200">WhatsApp Conectado</h4>
                        <p className="text-xs text-slate-400 max-w-xs">
                          Tu bot está activo escuchando mensajes y respondiendo automáticamente con Gemini Flash.
                        </p>
                      </div>
                    ) : (
                      <div className="py-8 text-center space-y-3">
                        <div className="w-16 h-16 bg-slate-800 rounded-full flex items-center justify-center mx-auto text-slate-500">
                          <XCircle className="w-8 h-8" />
                        </div>
                        <h4 className="font-medium text-slate-300">Desconectado</h4>
                        <p className="text-xs text-slate-400 max-w-xs">
                          Inicia el cliente para generar un código QR y conectar tu bot.
                        </p>
                      </div>
                    )}
                  </div>

                  <div className="mt-6 pt-4 border-t border-slate-800 flex gap-2">
                    {waStatus !== 'connected' ? (
                      <button
                        onClick={handleConnectWhatsApp}
                        className="flex-1 bg-indigo-600 hover:bg-indigo-500 text-white font-medium py-2 rounded-xl text-sm transition shadow-lg shadow-indigo-600/20"
                      >
                        Conectar WhatsApp
                      </button>
                    ) : (
                      <>
                        <button
                          onClick={handleDisconnectWhatsApp}
                          className="flex-1 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium py-2 rounded-xl text-xs transition"
                        >
                          Pausar
                        </button>
                        <button
                          onClick={handleLogoutWhatsApp}
                          className="flex-1 bg-red-600/20 border border-red-500/30 hover:bg-red-600/30 text-red-400 font-medium py-2 rounded-xl text-xs transition"
                        >
                          Cerrar Sesión
                        </button>
                      </>
                    )}
                  </div>
                </div>

                {/* Gemini Engine Info Card */}
                <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-xl space-y-3">
                  <div className="flex items-center gap-2 text-sm font-semibold text-slate-200">
                    <Sparkles className="w-4 h-4 text-amber-400" />
                    <span>Google AI Studio Failover</span>
                  </div>
                  <div className="text-xs text-slate-400 space-y-2">
                    <div className="flex justify-between items-center py-1 border-b border-slate-800/60">
                      <span>Modelo Activo Dinámico:</span>
                      <span className="text-indigo-400 font-mono font-medium">{cheapestModel}</span>
                    </div>
                    <div className="flex justify-between items-center py-1 border-b border-slate-800/60">
                      <span>Criterio de Costo:</span>
                      <span className="text-emerald-400">Flash de Menor Costo</span>
                    </div>
                    <div className="flex justify-between items-center py-1">
                      <span>Llaves en Rotación:</span>
                      <span className="text-slate-200 font-semibold">{config.gemini_api_keys.length} API Keys</span>
                    </div>
                  </div>
                </div>
              </div>

              {/* Chat & Live Log Box */}
              <div className="lg:col-span-8 flex flex-col space-y-6">
                <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl flex-1 flex flex-col min-h-[500px]">
                  <div className="flex items-center justify-between pb-4 border-b border-slate-800">
                    <div>
                      <h3 className="font-semibold text-slate-100">Historial de Conversaciones en Vivo</h3>
                      <p className="text-xs text-slate-400">Mensajes procesados y respuestas generadas por Gemini</p>
                    </div>
                    <span className="text-xs font-mono bg-slate-800 px-2 py-1 rounded text-slate-400">
                      {messages.length} mensajes
                    </span>
                  </div>

                  <div className="flex-1 overflow-y-auto space-y-4 py-4 max-h-[360px] pr-2">
                    {messages.length === 0 ? (
                      <div className="h-full flex flex-col items-center justify-center text-slate-500 py-16">
                        <Bot className="w-10 h-10 mb-2 opacity-40" />
                        <p className="text-sm">Aún no hay mensajes recibidos.</p>
                        <p className="text-xs text-slate-600 mt-1">
                          Los mensajes de WhatsApp aparecerán aquí en tiempo real.
                        </p>
                      </div>
                    ) : (
                      messages.map((m) => (
                        <div
                          key={m.id}
                          className={`flex flex-col ${m.isBot ? 'items-end' : 'items-start'}`}
                        >
                          <div className="text-[11px] text-slate-400 mb-1 flex items-center gap-2">
                            <span>{m.fromName || m.from}</span>
                            {m.modelUsed && (
                              <span className="bg-indigo-950 text-indigo-300 border border-indigo-800/50 px-1.5 py-0.2 rounded text-[10px]">
                                {m.modelUsed}
                              </span>
                            )}
                            {m.keyUsed && (
                              <span className="text-slate-500 text-[10px]">{m.keyUsed}</span>
                            )}
                          </div>
                          <div
                            className={`max-w-md rounded-2xl px-4 py-2.5 text-sm ${
                              m.isBot
                                ? 'bg-indigo-600 text-white rounded-br-none shadow-md shadow-indigo-600/10'
                                : 'bg-slate-800 text-slate-200 rounded-bl-none border border-slate-700/60'
                            }`}
                          >
                            <p className="whitespace-pre-wrap">{m.text}</p>
                          </div>
                        </div>
                      ))
                    )}
                  </div>

                  {/* Manual Dispatcher */}
                  <form onSubmit={handleSendManual} className="pt-4 border-t border-slate-800 mt-auto">
                    <div className="flex flex-col sm:flex-row gap-2">
                      <input
                        type="text"
                        placeholder="Número (ej: 51999999999)"
                        value={manualRecipient}
                        onChange={(e) => setManualRecipient(e.target.value)}
                        className="bg-slate-800/80 border border-slate-700 rounded-xl px-3 py-2 text-xs text-slate-200 placeholder-slate-500 sm:w-48 focus:outline-none focus:border-indigo-500"
                      />
                      <input
                        type="text"
                        placeholder="Enviar mensaje manual desde el panel..."
                        value={manualMessage}
                        onChange={(e) => setManualMessage(e.target.value)}
                        className="flex-1 bg-slate-800/80 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500"
                      />
                      <button
                        type="submit"
                        className="bg-indigo-600 hover:bg-indigo-500 text-white px-4 py-2 rounded-xl text-xs font-medium transition flex items-center gap-1.5 justify-center shadow-lg shadow-indigo-600/20"
                      >
                        <Send className="w-3.5 h-3.5" />
                        <span>Enviar</span>
                      </button>
                    </div>
                  </form>
                </div>
              </div>
            </div>
          )}

          {/* PDF MANAGER TAB */}
          {activeTab === 'pdf' && (
            <div className="max-w-5xl mx-auto space-y-8">
              {/* Action Selector */}
              <div className="flex bg-slate-900 p-1.5 rounded-2xl border border-slate-800 max-w-md mx-auto">
                <button
                  onClick={() => { setPdfAction('merge'); setPdfResult(null); }}
                  className={`flex-1 py-2 rounded-xl text-xs font-semibold flex items-center justify-center gap-2 transition ${
                    pdfAction === 'merge' ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-600/20' : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Layers className="w-4 h-4" />
                  <span>Unir PDFs</span>
                </button>
                <button
                  onClick={() => { setPdfAction('split'); setPdfResult(null); }}
                  className={`flex-1 py-2 rounded-xl text-xs font-semibold flex items-center justify-center gap-2 transition ${
                    pdfAction === 'split' ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-600/20' : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Scissors className="w-4 h-4" />
                  <span>Dividir PDF</span>
                </button>
                <button
                  onClick={() => { setPdfAction('compress'); setPdfResult(null); }}
                  className={`flex-1 py-2 rounded-xl text-xs font-semibold flex items-center justify-center gap-2 transition ${
                    pdfAction === 'compress' ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-600/20' : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Minimize2 className="w-4 h-4" />
                  <span>Comprimir</span>
                </button>
              </div>

              {/* Upload Card */}
              <div className="bg-slate-900 border border-slate-800 rounded-3xl p-8 shadow-2xl">
                <div className="text-center max-w-md mx-auto mb-6">
                  <h3 className="text-xl font-bold text-slate-100">
                    {pdfAction === 'merge' && 'Unir varios documentos PDF en uno solo'}
                    {pdfAction === 'split' && 'Dividir un PDF por páginas'}
                    {pdfAction === 'compress' && 'Optimizar y reducir el tamaño de un PDF'}
                  </h3>
                  <p className="text-xs text-slate-400 mt-1">
                    Procesado de forma local y segura mediante motor Go nativo.
                  </p>
                </div>

                <div className="border-2 border-dashed border-slate-700/80 hover:border-indigo-500/80 rounded-2xl p-8 text-center cursor-pointer transition bg-slate-950/40 relative">
                  <Upload className="w-12 h-12 text-indigo-400 mx-auto mb-3" />
                  {pdfAction === 'merge' ? (
                    <div>
                      <label className="cursor-pointer">
                        <span className="text-sm font-semibold text-indigo-400 hover:text-indigo-300">
                          Selecciona múltiples archivos PDF
                        </span>
                        <input
                          type="file"
                          multiple
                          accept=".pdf"
                          onChange={(e) => setPdfFiles(e.target.files)}
                          className="hidden"
                        />
                      </label>
                      <p className="text-xs text-slate-500 mt-1">
                        {pdfFiles ? `${pdfFiles.length} archivos seleccionados` : 'Arrastra o haz clic aquí'}
                      </p>
                    </div>
                  ) : (
                    <div>
                      <label className="cursor-pointer">
                        <span className="text-sm font-semibold text-indigo-400 hover:text-indigo-300">
                          Selecciona un archivo PDF
                        </span>
                        <input
                          type="file"
                          accept=".pdf"
                          onChange={(e) => setPdfSingleFile(e.target.files ? e.target.files[0] : null)}
                          className="hidden"
                        />
                      </label>
                      <p className="text-xs text-slate-500 mt-1">
                        {pdfSingleFile ? pdfSingleFile.name : 'Arrastra o haz clic aquí'}
                      </p>
                    </div>
                  )}
                </div>

                {pdfAction === 'split' && (
                  <div className="mt-4 flex items-center justify-center gap-3">
                    <label className="text-xs text-slate-300">Dividir cada N páginas:</label>
                    <input
                      type="number"
                      min="1"
                      value={splitSpan}
                      onChange={(e) => setSplitSpan(parseInt(e.target.value) || 1)}
                      className="w-20 bg-slate-800 border border-slate-700 rounded-lg px-2 py-1 text-xs text-center text-slate-100"
                    />
                  </div>
                )}

                <div className="mt-6 flex justify-center">
                  <button
                    disabled={pdfLoading}
                    onClick={handlePdfProcess}
                    className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white font-medium px-8 py-3 rounded-xl text-sm transition shadow-lg shadow-indigo-600/30 flex items-center gap-2"
                  >
                    {pdfLoading && <RefreshCw className="w-4 h-4 animate-spin" />}
                    <span>Procesar PDF</span>
                  </button>
                </div>

                {/* PDF Output Box */}
                {pdfResult && (
                  <div className="mt-8 pt-6 border-t border-slate-800 bg-slate-800/40 p-5 rounded-2xl">
                    <h4 className="text-sm font-semibold text-slate-200 mb-3 flex items-center gap-2">
                      <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                      Resultado del proceso:
                    </h4>

                    {pdfResult.type === 'merge' && (
                      <div className="flex items-center justify-between">
                        <span className="text-xs text-slate-300">{pdfResult.data.filename}</span>
                        <a
                          href={pdfResult.data.download_url}
                          download
                          className="bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-2 rounded-xl text-xs font-semibold flex items-center gap-2 transition"
                        >
                          <Download className="w-4 h-4" />
                          <span>Descargar PDF Unido</span>
                        </a>
                      </div>
                    )}

                    {pdfResult.type === 'split' && (
                      <div className="space-y-2">
                        <p className="text-xs text-slate-400">Páginas generadas:</p>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                          {pdfResult.data.files?.map((f: any, i: number) => (
                            <a
                              key={i}
                              href={f.url}
                              download
                              className="flex items-center justify-between bg-slate-800 p-2.5 rounded-xl text-xs hover:bg-slate-700 transition"
                            >
                              <span className="truncate max-w-[200px] text-slate-200">{f.name}</span>
                              <Download className="w-3.5 h-3.5 text-indigo-400" />
                            </a>
                          ))}
                        </div>
                      </div>
                    )}

                    {pdfResult.type === 'compress' && (
                      <div className="flex items-center justify-between">
                        <div className="text-xs space-y-1">
                          <p className="text-slate-200 font-medium">{pdfResult.data.filename}</p>
                          <p className="text-slate-400">
                            Original: {(pdfResult.data.original_size / 1024).toFixed(1)} KB → Comprimido:{' '}
                            {(pdfResult.data.compressed_size / 1024).toFixed(1)} KB
                          </p>
                        </div>
                        <a
                          href={pdfResult.data.download_url}
                          download
                          className="bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-2 rounded-xl text-xs font-semibold flex items-center gap-2 transition"
                        >
                          <Download className="w-4 h-4" />
                          <span>Descargar Optimizado</span>
                        </a>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* VIDEO DOWNLOADER TAB */}
          {activeTab === 'video' && (
            <div className="max-w-5xl mx-auto space-y-8">
              <div className="bg-slate-900 border border-slate-800 rounded-3xl p-8 shadow-2xl">
                <div className="max-w-xl mx-auto text-center mb-6">
                  <h3 className="text-xl font-bold text-slate-100">
                    Descargar Videos o Audio de YouTube y +1000 Plataformas
                  </h3>
                  <p className="text-xs text-slate-400 mt-1">
                    Impulsado por motor de alto rendimiento yt-dlp con soporte de transcodificación FFmpeg.
                  </p>
                </div>

                <div className="flex gap-2 max-w-2xl mx-auto">
                  <input
                    type="text"
                    placeholder="Pega la URL del video (YouTube, Facebook, Twitter, TikTok, etc.)..."
                    value={videoUrl}
                    onChange={(e) => handleVideoUrlChange(e.target.value)}
                    className="flex-1 bg-slate-800 border border-slate-700 rounded-xl px-4 py-3 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-indigo-500 font-mono"
                  />
                  <button
                    disabled={fetchingInfo || !videoUrl}
                    onClick={handleFetchVideoInfo}
                    className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white px-5 rounded-xl text-xs font-semibold transition flex items-center gap-2"
                  >
                    {fetchingInfo ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
                    <span>Examinar</span>
                  </button>
                </div>

                {/* Video Info Preview */}
                {videoInfo && (
                  <div className="mt-8 bg-slate-800/40 border border-slate-800 rounded-2xl p-6 flex flex-col md:flex-row gap-6 max-w-3xl mx-auto items-center">
                    {videoInfo.thumbnail && (
                      <img
                        src={videoInfo.thumbnail}
                        alt="Miniatura"
                        className="w-48 h-32 object-cover rounded-xl shadow-md"
                      />
                    )}
                    <div className="flex-1 space-y-3">
                      <h4 className="font-semibold text-slate-100 text-sm leading-snug line-clamp-2">
                        {videoInfo.title}
                      </h4>
                      <p className="text-xs text-slate-400">Canal / Creador: {videoInfo.uploader || 'Desconocido'}</p>

                      <div className="flex items-center gap-4 pt-2">
                        <select
                          value={downloadFormat}
                          onChange={(e: any) => setDownloadFormat(e.target.value)}
                          className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-xs text-slate-200"
                        >
                          <option value="video">Video (MP4)</option>
                          <option value="audio">Solo Audio (MP3)</option>
                        </select>

                        {downloadFormat === 'video' && (
                          <select
                            value={downloadQuality}
                            onChange={(e) => setDownloadQuality(e.target.value)}
                            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-xs text-slate-200"
                          >
                            <option value="best">Máxima Calidad</option>
                            <option value="1080p">1080p Full HD</option>
                            <option value="720p">720p HD</option>
                          </select>
                        )}

                        <button
                          onClick={handleStartDownload}
                          className="bg-emerald-600 hover:bg-emerald-500 text-white px-4 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition ml-auto shadow-md shadow-emerald-600/20"
                        >
                          <Download className="w-3.5 h-3.5" />
                          <span>Descargar</span>
                        </button>
                      </div>
                    </div>
                  </div>
                )}

                {/* Active Jobs */}
                <div className="mt-8">
                  <h4 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3">
                    Cola de Descargas
                  </h4>
                  <div className="space-y-2">
                    {jobs.length === 0 ? (
                      <p className="text-xs text-slate-600 italic">No hay descargas activas o recientes.</p>
                    ) : (
                      jobs.map((job) => (
                        <div
                          key={job.id}
                          className="bg-slate-800/60 border border-slate-800 p-3.5 rounded-xl flex items-center justify-between"
                        >
                          <div className="space-y-1">
                            <p className="text-xs font-medium text-slate-200 truncate max-w-md">
                              {job.filename || job.url}
                            </p>
                            <p className="text-[11px] text-slate-400 flex items-center gap-2">
                              <span>Formato: {job.format.toUpperCase()}</span>
                              <span>•</span>
                              <span className="text-indigo-400 font-mono">{job.progress}</span>
                            </p>
                          </div>

                          <div>
                            {job.status === 'completed' && job.filepath && (
                              <a
                                href={`/downloads/${encodeURIComponent(job.filename)}`}
                                download
                                className="bg-indigo-600 hover:bg-indigo-500 text-white px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition"
                              >
                                <Download className="w-3.5 h-3.5" />
                                <span>Guardar Archivo</span>
                              </a>
                            )}
                            {job.status === 'downloading' && (
                              <span className="text-xs text-amber-400 flex items-center gap-1 font-mono">
                                <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                                <span>{job.progress}</span>
                              </span>
                            )}
                            {job.status === 'error' && (
                              <span className="text-xs text-red-400">Error: {job.error}</span>
                            )}
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* SETTINGS TAB */}
          {activeTab === 'settings' && (
            <div className="max-w-4xl mx-auto space-y-8">
              <div className="bg-slate-900 border border-slate-800 rounded-3xl p-8 shadow-2xl space-y-6">
                <div>
                  <h3 className="text-lg font-bold text-slate-100 flex items-center gap-2">
                    <Key className="w-5 h-5 text-indigo-400" />
                    Google AI Studio API Keys con Failover Inteligente
                  </h3>
                  <p className="text-xs text-slate-400 mt-1">
                    Puedes agregar múltiples API Keys de Google AI Studio. Si una clave agota su cuota de peticiones o falla, el sistema conmuta automáticamente (failover) a la siguiente clave sin interrumpir las respuestas de WhatsApp.
                  </p>
                </div>

                {/* API Key list */}
                <div className="space-y-2">
                  {config.gemini_api_keys.map((k, idx) => (
                    <div
                      key={idx}
                      className="flex items-center justify-between bg-slate-800/70 border border-slate-700/60 px-4 py-2.5 rounded-xl"
                    >
                      <div className="flex items-center gap-3">
                        <span className="text-xs font-bold text-indigo-400">#{idx + 1}</span>
                        <span className="font-mono text-xs text-slate-300">
                          {k.length > 12 ? `${k.slice(0, 6)}••••••••••••${k.slice(-4)}` : k}
                        </span>
                      </div>
                      <button
                        onClick={() => handleRemoveKey(idx)}
                        className="text-slate-500 hover:text-red-400 p-1 transition"
                        title="Eliminar llave"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  ))}

                  {/* Add key input */}
                  <div className="flex gap-2 pt-2">
                    <input
                      type="password"
                      placeholder="Pega aquí tu API Key de Google AI Studio (AIzaSy...)"
                      value={newKey}
                      onChange={(e) => setNewKey(e.target.value)}
                      className="flex-1 bg-slate-800 border border-slate-700 rounded-xl px-4 py-2.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500 font-mono"
                    />
                    <button
                      onClick={handleAddKey}
                      className="bg-indigo-600 hover:bg-indigo-500 text-white px-4 py-2.5 rounded-xl text-xs font-semibold flex items-center gap-1.5 transition shadow-lg shadow-indigo-600/20"
                    >
                      <Plus className="w-4 h-4" />
                      <span>Agregar Key</span>
                    </button>
                  </div>
                </div>

                {/* System Prompt */}
                <div className="pt-4 border-t border-slate-800 space-y-2">
                  <label className="text-xs font-semibold text-slate-300">
                    Instrucción de Sistema para el Chatbot (System Prompt)
                  </label>
                  <textarea
                    rows={4}
                    value={config.system_prompt}
                    onChange={(e) => setConfig({ ...config, system_prompt: e.target.value })}
                    className="w-full bg-slate-800 border border-slate-700 rounded-xl p-3 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500 font-mono"
                    placeholder="Define la personalidad, reglas y comportamiento del bot de WhatsApp..."
                  />
                </div>

                {/* Auto Reply toggle */}
                <div className="flex items-center justify-between pt-2">
                  <div>
                    <h5 className="text-sm font-medium text-slate-200">Respuesta Automática por WhatsApp</h5>
                    <p className="text-xs text-slate-500">
                      Cuando está activo, el bot responderá de forma autónoma a los mensajes recibidos.
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    checked={config.auto_reply}
                    onChange={(e) => setConfig({ ...config, auto_reply: e.target.checked })}
                    className="w-5 h-5 accent-indigo-600 rounded cursor-pointer"
                  />
                </div>

                <div className="pt-4 flex items-center justify-between">
                  <button
                    onClick={handleSaveConfig}
                    className="bg-emerald-600 hover:bg-emerald-500 text-white font-medium px-6 py-2.5 rounded-xl text-xs transition shadow-lg shadow-emerald-600/20"
                  >
                    Guardar Configuración
                  </button>
                  {saveSuccess && (
                    <span className="text-xs text-emerald-400 font-medium flex items-center gap-1">
                      <CheckCircle2 className="w-4 h-4" /> ¡Configuración guardada exitosamente!
                    </span>
                  )}
                </div>
              </div>

              {/* Gemini Sandbox / Failover Tester */}
              <div className="bg-slate-900 border border-slate-800 rounded-3xl p-8 shadow-2xl space-y-4">
                <div className="flex items-center gap-2">
                  <Terminal className="w-5 h-5 text-indigo-400" />
                  <h4 className="font-semibold text-sm text-slate-200">Probador de Failover de Gemini Flash</h4>
                </div>
                <p className="text-xs text-slate-400">
                  Prueba una respuesta directa con tus llaves y verifica dinámicamente qué modelo Flash de bajo costo y qué API key atienden la solicitud.
                </p>

                <div className="flex gap-2">
                  <input
                    type="text"
                    value={testPrompt}
                    onChange={(e) => setTestPrompt(e.target.value)}
                    placeholder="Mensaje de prueba..."
                    className="flex-1 bg-slate-800 border border-slate-700 rounded-xl px-4 py-2 text-xs text-slate-200"
                  />
                  <button
                    disabled={testingGemini}
                    onClick={testGeminiFailover}
                    className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white px-5 py-2 rounded-xl text-xs font-semibold flex items-center gap-2 transition"
                  >
                    {testingGemini ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
                    <span>Probar</span>
                  </button>
                </div>

                {testResponse && (
                  <div className="bg-slate-950 p-4 rounded-xl border border-slate-800 text-xs space-y-2">
                    {testResponse.error ? (
                      <div className="text-red-400 font-mono">Error: {testResponse.error}</div>
                    ) : (
                      <>
                        <div className="flex gap-4 text-slate-400 font-mono text-[11px] pb-2 border-b border-slate-800">
                          <span>
                            Modelo usado:{' '}
                            <b className="text-indigo-400">{testResponse.model_used}</b>
                          </span>
                          <span>
                            Clave usada:{' '}
                            <b className="text-emerald-400">{testResponse.key_used}</b>
                          </span>
                        </div>
                        <p className="text-slate-200 whitespace-pre-wrap">{testResponse.reply}</p>
                      </>
                    )}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

export default App;
