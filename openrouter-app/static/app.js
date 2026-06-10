const STORAGE_KEY = "ipyintervu_public_key";

const messagesEl = document.getElementById("messages");
const chatForm = document.getElementById("chat-form");
const promptEl = document.getElementById("prompt");
const modelEl = document.getElementById("model");
const sendBtn = document.getElementById("send-btn");
const clearBtn = document.getElementById("clear-btn");
const appEl = document.querySelector(".app");
const authGateEl = document.getElementById("auth-gate");
const authFormEl = document.getElementById("auth-form");
const publicKeyInputEl = document.getElementById("public-key-input");
const authErrorEl = document.getElementById("auth-error");
const authSubmitBtn = document.getElementById("auth-submit-btn");

const conversation = [];
let isAuthenticated = false;

const INSECURE_CONTEXT_MESSAGE =
  "Authentication requires HTTPS or localhost. Browsers block encryption on plain HTTP to remote hosts. Use https://, open http://127.0.0.1/ on the server, or use an SSH tunnel.";

function hasSecureCrypto() {
  return window.isSecureContext && window.crypto && window.crypto.subtle;
}

function normalizePublicKey(pem) {
  return pem.replace(/\r/g, "").trim();
}

function setChatEnabled(enabled) {
  isAuthenticated = enabled;
  promptEl.disabled = !enabled;
  sendBtn.disabled = !enabled;
  clearBtn.disabled = !enabled;
  modelEl.disabled = !enabled;
  appEl.classList.toggle("app-locked", !enabled);
}

function showAuthGate(message = "") {
  authGateEl.classList.remove("hidden");
  authGateEl.setAttribute("aria-hidden", "false");
  if (message) {
    authErrorEl.textContent = message;
    authErrorEl.classList.remove("hidden");
  } else {
    authErrorEl.textContent = "";
    authErrorEl.classList.add("hidden");
  }
  setChatEnabled(false);
}

function hideAuthGate() {
  authGateEl.classList.add("hidden");
  authGateEl.setAttribute("aria-hidden", "true");
  authErrorEl.textContent = "";
  authErrorEl.classList.add("hidden");
  setChatEnabled(true);
}

function pemToArrayBuffer(pem) {
  const base64 = pem
    .replace(/-----BEGIN PUBLIC KEY-----/g, "")
    .replace(/-----END PUBLIC KEY-----/g, "")
    .replace(/\s/g, "");
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

function bufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary);
}

async function importPublicKey(pem) {
  if (!hasSecureCrypto()) {
    throw new Error(INSECURE_CONTEXT_MESSAGE);
  }

  return crypto.subtle.importKey(
    "spki",
    pemToArrayBuffer(normalizePublicKey(pem)),
    { name: "RSA-OAEP", hash: "SHA-256" },
    false,
    ["encrypt"]
  );
}

async function encryptChallenge(publicKeyPem, nonceBase64) {
  const key = await importPublicKey(publicKeyPem);
  const nonceBytes = Uint8Array.from(atob(nonceBase64), (char) => char.charCodeAt(0));
  const ciphertext = await crypto.subtle.encrypt({ name: "RSA-OAEP" }, key, nonceBytes);
  return bufferToBase64(ciphertext);
}

async function authenticate(publicKeyPem) {
  const challengeResponse = await fetch("/api/auth/challenge", { credentials: "include" });
  if (!challengeResponse.ok) {
    throw new Error("Failed to get authentication challenge.");
  }

  const { challenge_id: challengeID, nonce } = await challengeResponse.json();
  const ciphertext = await encryptChallenge(publicKeyPem, nonce);

  const verifyResponse = await fetch("/api/auth/verify", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      challenge_id: challengeID,
      ciphertext,
    }),
  });

  if (!verifyResponse.ok) {
    throw new Error("Authentication failed.");
  }
}

async function initAuth() {
  setChatEnabled(false);

  if (!hasSecureCrypto()) {
    showAuthGate(INSECURE_CONTEXT_MESSAGE);
    return;
  }

  const storedKey = localStorage.getItem(STORAGE_KEY);
  if (storedKey) {
    try {
      await authenticate(storedKey);
      hideAuthGate();
      return;
    } catch {
      localStorage.removeItem(STORAGE_KEY);
    }
  }

  showAuthGate();
}

function renderMessages() {
  messagesEl.innerHTML = "";

  if (conversation.length === 0) {
    const empty = document.createElement("div");
    empty.className = "message empty";
    empty.textContent = "Start a conversation by sending a message.";
    messagesEl.appendChild(empty);
    return;
  }

  for (const message of conversation) {
    const item = document.createElement("div");
    item.className = `message ${message.role}`;
    item.textContent = message.content;
    messagesEl.appendChild(item);
  }

  messagesEl.scrollTop = messagesEl.scrollHeight;
}

function appendAssistantDelta(delta) {
  const last = conversation[conversation.length - 1];
  if (!last || last.role !== "assistant") {
    conversation.push({ role: "assistant", content: delta });
  } else {
    last.content += delta;
  }
  renderMessages();
}

function showError(text) {
  conversation.push({ role: "error", content: text });
  renderMessages();
}

function parseSSEChunk(chunk, onDelta) {
  const lines = chunk.split("\n");

  for (const line of lines) {
    if (!line.startsWith("data: ")) {
      continue;
    }

    const data = line.slice(6).trim();
    if (!data || data === "[DONE]") {
      continue;
    }

    let payload;
    try {
      payload = JSON.parse(data);
    } catch {
      continue;
    }

    const delta = payload.choices?.[0]?.delta?.content;
    if (delta) {
      onDelta(delta);
    }
  }
}

async function sendMessage(text) {
  if (!isAuthenticated) {
    throw new Error("Authentication required.");
  }

  conversation.push({ role: "user", content: text });
  renderMessages();

  const payload = {
    model: modelEl.value,
    messages: conversation
      .filter((message) => message.role === "user" || message.role === "assistant")
      .map(({ role, content }) => ({ role, content })),
    stream: true,
  };

  const response = await fetch("/api/chat", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (response.status === 401) {
    localStorage.removeItem(STORAGE_KEY);
    showAuthGate("Your session expired or is invalid. Enter your public key again.");
    throw new Error("Authentication required.");
  }

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(errorText || `Request failed with status ${response.status}`);
  }

  const contentType = response.headers.get("Content-Type") || "";

  if (!contentType.includes("text/event-stream")) {
    const data = await response.json();
    const content = data.choices?.[0]?.message?.content || "";
    conversation.push({ role: "assistant", content });
    renderMessages();
    return;
  }

  conversation.push({ role: "assistant", content: "" });
  renderMessages();

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop() || "";

    for (const part of parts) {
      parseSSEChunk(part, appendAssistantDelta);
    }
  }

  if (buffer) {
    parseSSEChunk(buffer, appendAssistantDelta);
  }

  const last = conversation[conversation.length - 1];
  if (last?.role === "assistant" && !last.content) {
    conversation.pop();
    renderMessages();
  }
}

authFormEl.addEventListener("submit", async (event) => {
  event.preventDefault();

  const publicKey = normalizePublicKey(publicKeyInputEl.value);
  if (!publicKey) {
    showAuthGate("Enter a public key to continue.");
    return;
  }

  authSubmitBtn.disabled = true;

  try {
    await authenticate(publicKey);
    localStorage.setItem(STORAGE_KEY, publicKey);
    hideAuthGate();
    promptEl.focus();
  } catch (error) {
    showAuthGate(error.message || "Invalid public key or authentication failed.");
  } finally {
    authSubmitBtn.disabled = false;
  }
});

chatForm.addEventListener("submit", async (event) => {
  event.preventDefault();

  const text = promptEl.value.trim();
  if (!text || !isAuthenticated) {
    return;
  }

  promptEl.value = "";
  sendBtn.disabled = true;
  clearBtn.disabled = true;

  try {
    await sendMessage(text);
  } catch (error) {
    if (isAuthenticated) {
      showError(error.message || "Something went wrong.");
    }
  } finally {
    if (isAuthenticated) {
      sendBtn.disabled = false;
      clearBtn.disabled = false;
      promptEl.focus();
    }
  }
});

clearBtn.addEventListener("click", () => {
  if (!isAuthenticated) {
    return;
  }
  conversation.length = 0;
  renderMessages();
  promptEl.focus();
});

renderMessages();
initAuth();
