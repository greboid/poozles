const root = document.getElementById('input')
if (root) {
  const hints = document.getElementById('hints')
  const guesses = document.getElementById('guesses')
  root.onsubmit = async (event) => {
    event.preventDefault()
    const now = new Date()
    const formData= new FormData(event.target)
    const response = await fetch('/guess', {
      method: 'POST',
      body: formData
    }).then(res => res.json())
    if (guesses.innerHTML.length > 0) {
      guesses.innerHTML += "<br>"
    }
    guesses.innerText += now.getHours().toString().padStart(2, '0') + ":" + now.getMinutes() + " - " + response.guess
    if (response.result === 'correct') {
      root.classList.remove('error')
      root.classList.add('correct')
    } else if (response.result === 'incorrect') {
      root.classList.remove('correct')
      root.classList.add('error')
    } else if (response.result === 'unlock') {
      if (hints.innerHTML.length > 0) {
        hints.innerHTML += "<br>"
      }
      guesses.innerText += " [Unlock]"
      hints.innerText += now.getHours().toString().padStart(2, '0') + ":" + now.getMinutes() + " - " + response.guess + " => " + response.unlock
    } else {
      alert('wtf')
      console.log(response)
    }
  }
  root.oninput = (event) => {
    root.classList.remove('error')
    root.classList.remove('correct')
  }
}
