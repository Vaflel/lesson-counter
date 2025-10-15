document.addEventListener('DOMContentLoaded', () => {
  // Обработчик формы проверки расписания (если форма есть на странице)
  const checkForm = document.getElementById('checkForm');
  if (checkForm) {
    checkForm.addEventListener('submit', async function(event) {
      event.preventDefault();

      const weekStart = document.getElementById('weekStart').value;
      const spinner = document.getElementById('spinner');
      const resultDiv = document.getElementById('result');
      const submitButton = document.querySelector('#checkForm button');

      if (spinner) spinner.style.display = 'block';
      if (submitButton) {
        submitButton.disabled = true;
        submitButton.textContent = 'Проверка выполняется...';
      }
      if (resultDiv) resultDiv.innerHTML = '';

      try {
        const response = await fetch('/check', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ weekStart }),
        });

        const data = await response.json();

        if (!data.success) {
          if (resultDiv) resultDiv.innerHTML = `<p style="color: red;">Ошибка: ${data.message}</p>`;
          if (spinner) spinner.style.display = 'none';
          if (submitButton) {
            submitButton.disabled = false;
            submitButton.textContent = 'Проверить расписание';
          }
          return;
        }

        const checkStatus = async () => {
          const statusResponse = await fetch('/status');
          const statusData = await statusResponse.json();

          if (!statusData.isProcessing && statusData.reportReady) {
            if (resultDiv) resultDiv.innerHTML = statusData.report || '<p>Ошибка: отчет не получен.</p>';
            if (spinner) spinner.style.display = 'none';
            if (submitButton) {
              submitButton.disabled = false;
              submitButton.textContent = 'Проверить расписание';
            }
          } else {
            setTimeout(checkStatus, 1000);
          }
        };

        checkStatus();
      } catch (error) {
        if (resultDiv) resultDiv.innerHTML = `<p style="color: red;">Ошибка при выполнении запроса: ${error.message}</p>`;
        if (spinner) spinner.style.display = 'none';
        if (submitButton) {
          submitButton.disabled = false;
          submitButton.textContent = 'Проверить расписание';
        }
      }
    });
  }

  // Обработчик кнопки завершения (если кнопка есть на странице)
  const shutdownButton = document.getElementById('shutdownButton');
  if (shutdownButton) {
    shutdownButton.addEventListener('click', async function(event) {
      event.preventDefault();
      if (!confirm('Вы уверены, что хотите завершить работу приложения?')) return;
      try {
        const response = await fetch('/shutdown', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
        });
        const data = await response.json();
        alert(data.success ? 'Приложение завершило работу.' : ('Ошибка: ' + data.message));
      } catch (error) {
        alert('Ошибка при выполнении запроса: ' + error.message);
      }
    });
  }

  // Обработчик кнопок удаления студентов
  const deleteButtons = document.querySelectorAll('.delete-student');
  deleteButtons.forEach(button => {
    button.addEventListener('click', async (event) => {
      event.preventDefault();
      const url = button.getAttribute('href');
      const studentName = url.split('/').pop();
      
      if (!confirm(`Вы уверены, что хотите удалить студента ${decodeURIComponent(studentName)}?`)) {
        return;
      }

      try {
        const response = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
        });

        if (response.ok) {
          window.location.href = '/students'; // Перенаправление после успешного удаления
        } else {
          const data = await response.json();
          alert(`Ошибка: ${data.message || 'Не удалось удалить студента'}`);
        }
      } catch (error) {
        alert(`Ошибка при выполнении запроса: ${error.message}`);
      }
    });
  });
});