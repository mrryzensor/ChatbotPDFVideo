# 🚀 OmniSaaS Hub (WhatsApp Bot IA, Gestor de PDFs y Descargador de Videos)

Una suite todo-en-uno moderna y profesional con interfaz SaaS creada en **Go + React + Tailwind CSS**.

![Dashboard](https://raw.githubusercontent.com/mrryzensor/ChatbotPDFVideo/main/frontend/src/assets/hero.png)

---

## ⚡ Inicio Rápido (En 1 Clic para Todos)

> **No necesitas conocimientos técnicos previos.** La aplicación incluye un instalador automático que se encarga de todo.

1. **Descarga el repositorio:**
   - Haz clic en el botón verde **Code** (arriba a la derecha) y luego en **Download ZIP**.
   - Descomprime la carpeta en tu computadora.
2. **Haz doble clic en el archivo:**
   ```text
   INICIAR_APP.bat
   ```
3. **¡Listo!** El script verificará las herramientas necesarias, compilará el sistema y abrirá automáticamente tu navegador en:
   👉 **http://localhost:8080**

---

## 🌟 Características Principales

### 1. 🤖 Chatbot de WhatsApp Inteligente
- **Motor WhatsMeow en Go**: Conexión nativa, estable y rápida sin depender de navegadores externos pesados como Puppeteer.
- **Código QR en Pantalla**: Escanea el código con tu celular desde *WhatsApp > Dispositivos vinculados*.
- **Google AI Studio con Failover Inteligente**: Puedes ingresar múltiples API Keys de Google Gemini. Si una llave agota su cuota o tiene límites de tasa, pasa de inmediato a la siguiente clave sin perder ningún mensaje.
- **Elección dinámica del modelo más barato**: Detecta automáticamente el modelo más económico disponible en tu cuenta (`gemini-3.5-flash-lite`, `gemini-3.5-flash`, etc.).
- **Filtro de Privacidad**: El bot **ignora grupos y transmisiones**, respondiendo únicamente a chats individuales directos.
- **Consola de Mensajes**: Historial en tiempo real y panel para enviar mensajes directos a cualquier número.

### 2. 📄 Gestor Avanzado de PDFs
- **Unir PDFs (Merge)**: Sube dos o más archivos y combínalos en un único PDF ordenado.
- **Dividir PDF (Split)**: Separa documentos grandes en partes de $N$ páginas con enlaces de descarga directa.
- **Comprimir PDF (Compress)**: Reduce el peso del documento optimizando la estructura interna sin sacrificar legibilidad.

### 3. 🎥 Descargador Multimedia Universal
- **Impulsado por `yt-dlp` y `FFmpeg`**: Compatible con YouTube, TikTok, Facebook, Instagram, Twitter (X) y +1000 sitios web.
- **Normalizador de Enlaces**: Limpia códigos de seguimiento (`?si=...`, `&pp=...`, etc.) y traduce automáticamente enlaces de Shorts o `youtu.be`.
- **Selector de Calidad**:
  - Video MP4: Máxima calidad, 1080p Full HD o 720p HD.
  - Audio MP3: Extrae la música o audio directo del video.
- **Monitor en vivo**: Barra de progreso y botón de descarga directa para guardar en tu equipo.

---

## 🛠️ Ejecución Manual (Para Desarrolladores)

Si prefieres levantarlo manualmente paso a paso:

### Requisitos:
- **Go 1.22+**
- **Node.js 18+** y **npm**
- **yt-dlp** y **FFmpeg**

### Pasos:

1. **Compilar el Frontend (React + Vite + Tailwind):**
   ```bash
   cd frontend
   npm install
   npm run build
   cd ..
   ```

2. **Compilar y ejecutar el Backend en Go:**
   ```bash
   go build -o app.exe main.go
   .\app.exe
   ```

3. **Abrir en el navegador:**
   [http://localhost:8080](http://localhost:8080)

---

## 🔑 Obtener API Keys de Google AI Studio (Gratuitas)
1. Ingresa a [Google AI Studio](https://aistudio.google.com/).
2. Haz clic en **Get API key** y luego en **Create API key**.
3. Pégala en la pestaña **Configuración & API Keys** de tu aplicación y presiona **+ Agregar Key**.

---

## 📄 Licencia
Distribuido bajo la licencia MIT. Consulta `LICENSE` para más información.
