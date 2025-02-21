document.addEventListener("DOMContentLoaded", function () {
  var inputField = document.querySelector('.shortify');
  if (inputField) {
    inputField.value = "";
  }
});

document.getElementById('linkForm').addEventListener('submit', function (event) {
  event.preventDefault();
  var form = event.target;
  var formData = new FormData(form);
  var params = new URLSearchParams(formData);

  fetch(form.action, {
    method: form.method,
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded'
    },
    body: params
  })
  .then(response => response.json())
  .then(data => {
    if (data.shortURL) {
      document.querySelector('.input').style.display = 'none';
      var resultDiv = document.getElementById('result');
      resultDiv.style.display = 'block';
      var resultH3 = resultDiv.querySelector('h3.result');
      if (resultH3) {
        resultH3.innerText = data.shortURL;
      }
      var copyButton = document.getElementById('copyButton');
      copyButton.style.display = 'block';
    }
  })
  .catch(error => {
    console.error('Error:', error);
  });
});

document.getElementById('copyButton').addEventListener('click', function () {
  var resultH3 = document.querySelector('h3.result');
  var shortURL = resultH3.innerText;
  navigator.clipboard.writeText(shortURL).then(function () {
    this.innerText = 'Copied!';
    var copyButton = this;
    setTimeout(function () {
      copyButton.innerText = 'Copy';
    }, 2000);
  }.bind(this)).catch(function (err) {
    console.error('Error copying to clipboard:', err);
  });
});
