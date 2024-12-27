const timeStamp = () => {
  const now = new Date()
  return now.getHours().toString().padStart(2, '0')
         + ':'
         + now.getMinutes().toString().padStart(2, '0')
}

const submitGuess = async (e) => {
  e.preventDefault()
  fetch('/guess', {
    method: 'POST',
    body:   new FormData(document.getElementById('input')),
  })
      .then(res => res.json())
      .then(response => handleGuessResponse(response))
}

const handleGuessResponse = (response) => {
  const input = document.getElementById('input')
  const guesses = document.getElementById('guesses')
  const unlocks = document.getElementById('unlocks')
  input.elements.guess.value = ''
  if (guesses.innerHTML.length > 0) {
    guesses.innerHTML += '<br>'
  }
  guesses.innerText += timeStamp() + ' - ' + response.guess
  if (response.result === 'correct') {
    input.classList.remove('error')
    input.classList.add('correct')
  } else if (response.result === 'incorrect') {
    input.classList.remove('correct')
    input.classList.add('error')
  } else if (response.result === 'unlock') {
    if (unlocks.innerHTML.length > 0) {
      unlocks.innerHTML += '<br>'
    }
    guesses.innerText += ' [Unlock]'
    unlocks.innerText += timeStamp() + ' - ' + response.guess + ' => ' + response.unlock
  } else {
    alert('wtf')
    console.log(response)
  }
}

const clearInput = () => {
  const input = document.getElementById('input')
  input.classList.remove('error')
  input.classList.remove('correct')
}

const revealHint = (event) => {
  const input = document.getElementById('input')
  const parent = event.target.closest('.hint')
  const hintId = parseInt(parent.dataset.index)
  const puzzle = input.elements.puzzle.value

  fetch('/hint', {
    method: 'POST',
    body:   JSON.stringify({puzzle: puzzle, hintRequested: hintId}),
  })
      .then(res => res.json())
      .then(response => {
        parent.querySelector('p').innerText = response.hint
        parent.querySelector('p').className = 'unlocked'
        event.target.remove()
      })
}

const addEventButtonListeners = (buttons) => {
  buttons.forEach((el) => el.addEventListener('click', revealHint))
}

if (document.getElementById('input')) {
  document.getElementById('input').onsubmit = submitGuess
  document.getElementById('input').oninput = clearInput
  addEventButtonListeners(document.querySelectorAll('.hint button'))
}
