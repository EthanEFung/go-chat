const parser = new DOMParser();
window.addEventListener("DOMContentLoaded", function() {
  const websocket = new WebSocket("ws://"+window.location.host+"/websocket")
  const room = document.querySelector("#chat-text")
  websocket.addEventListener("message", function(e) {
    const data = JSON.parse(e.data)
    const chatContent = `<p><strong>${data.username}</strong>: ${data.text}</p>`
    const d = parser.parseFromString(chatContent, 'text/html')
    room.append(d.body.firstChild)
  })
  const form = document.getElementById("input-form")
  form.addEventListener("submit", function(event) {
    event.preventDefault()
    const username = document.querySelector("#input-username").value
    const text = document.querySelector("#input-text").value
    websocket.send(
      JSON.stringify({
        username: username,
        text: text,
      })
    )
    text.value = ""
  })
})