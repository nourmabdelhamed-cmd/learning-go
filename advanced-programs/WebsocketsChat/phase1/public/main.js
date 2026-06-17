const statusEl = document.querySelector("#status");
const messagesEl = document.querySelector("#messages");
const formEl = document.querySelector("#chat-form");
const usernameEl = document.querySelector("#username");
const contentEl = document.querySelector("#content");

const protocol = window.location.protocol === "https:" ? "wss" : "ws";
const socket = new WebSocket(`${protocol}://${window.location.host}/ws`);

socket.addEventListener("open", () => {
  statusEl.textContent = "connected";
  statusEl.dataset.state = "connected";
});

socket.addEventListener("close", () => {
  statusEl.textContent = "disconnected";
  statusEl.dataset.state = "disconnected";
});

socket.addEventListener("error", () => {
  statusEl.textContent = "error";
  statusEl.dataset.state = "error";
});

socket.addEventListener("message", (event) => {
  const message = JSON.parse(event.data);
  renderMessage(message);
});

formEl.addEventListener("submit", (event) => {
  event.preventDefault();

  const payload = {
    type: "chat",
    username: usernameEl.value,
    content: contentEl.value,
  };

  socket.send(JSON.stringify(payload));
  contentEl.value = "";
  contentEl.focus();
});

function renderMessage(message) {
  const item = document.createElement("article");
  item.className = `message message-${message.type}`;

  const meta = document.createElement("div");
  meta.className = "message-meta";
  const timestamp = new Date(message.timestamp).toLocaleTimeString();
  if (message.type === "chat") {
    meta.textContent = `${message.username} · ${timestamp}`;
  } else {
    meta.textContent = `${message.type} · ${timestamp}`;
  }

  const content = document.createElement("div");
  content.className = "message-content";
  content.textContent = message.content;

  item.append(meta, content);
  messagesEl.prepend(item);
}
