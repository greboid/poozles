const root = document.getElementById('input')
if (root) {
  document.getElementById('input').onsubmit = async (event) => {
    event.preventDefault()
    const formData= new FormData(event.target)
    const response = await fetch('/guess', {
      method: 'POST',
      body: formData
    })
    if (response.status === 200) {
      alert('yay')
    } else if (response.status === 404) {
      alert('boo')
    } else {
      alert('wtf')
      console.log(response)
    }
    }
}
