package core

import "strings"

// ErrorHint classifies known tool messages; the original log stays available.
func ErrorHint(lines []string, russian bool) string {
	text := strings.ToLower(strings.Join(lines, "\n"))
	rules := []struct {
		needles []string
		ru, en  string
	}{
		{[]string{"no space left", "disk full"}, "Недостаточно места. Освободите место или выберите другую папку.", "Disk full. Free space or choose another folder."},
		{[]string{"permission denied", "access is denied"}, "Нет доступа к файлу. Проверьте права и папку назначения.", "Access denied. Check permissions and the output folder."},
		{[]string{"unknown encoder", "encoder not found"}, "Кодек недоступен в этой сборке FFmpeg. Выберите другой кодек или FFmpeg.", "Encoder unavailable in this FFmpeg build. Choose another encoder or FFmpeg."},
		{[]string{"sign in", "cookies", "login required", "http error 403"}, "Проверьте авторизацию и cookies браузера; при 403 также проверьте доступ к сайту.", "Check login and browser cookies; for HTTP 403 also check site access."},
		{[]string{"connection refused", "timed out", "temporary failure", "unable to download"}, "Проверьте сеть и настройки прокси, затем повторите задачу.", "Check network and proxy settings, then retry the task."},
		{[]string{"already exists", "not overwriting"}, "Выходной файл уже существует. Укажите другое имя.", "Output already exists. Choose another filename."},
		{[]string{"no such file", "not found"}, "Проверьте путь к исходнику и наличие необходимых программ.", "Check the input path and required tools."},
	}
	for _, rule := range rules {
		for _, needle := range rule.needles {
			if strings.Contains(text, needle) {
				if russian {
					return rule.ru
				}
				return rule.en
			}
		}
	}
	if russian {
		return "Операция завершилась с ошибкой. Причина указана в журнале; повтор доступен в очереди."
	}
	return "Operation failed. Check the log; retry is available in the queue."
}
