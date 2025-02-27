package config

import (
	"fmt"

	"github.com/robfig/cron/v3"
)

func StartCronJobs() {
	c := cron.New()
	// c.AddFunc("*/1 * * * *", promoteQueue)
	c.AddFunc("* * * * *", archiveOldApplications)
	c.AddFunc("* * * * *", updateQueueNumbersCron)
	c.AddFunc("* * * * *", updateRejectedAndArchived)
	c.Start()
}

// func promoteQueue() {
// 	_, err := DB.Exec(`
// 			UPDATE applications
// 			SET status = 'promoted', promoted_at = NOW()
// 			WHERE id IN (
// 				SELECT id FROM applications
// 				WHERE status = 'approved'
// 				ORDER BY queue_number ASC
// 				LIMIT 100
// 			)
// 		`)
// 	if err != nil {
// 		fmt.Println("Ошибка продвижения очереди:", err)
// 		return
// 	}

// 	fmt.Println("Продвинуто 100 человек в очереди")
// }

func archiveOldApplications() {
	_, err := DB.Exec(`
		UPDATE applications
		SET status = 'archive', deleted_at = NOW()
		WHERE status = 'promoted' AND promoted_at <= NOW() - INTERVAL '30 seconds'
	`)
	if err != nil {
		fmt.Println("Ошибка при обновлении статусов на archive:", err)
		return
	}

	fmt.Println("Успешно обновлены статусы на archive")
}

func updateQueueNumbersCron() {
	_, err := DB.Exec(`
		WITH Ranked AS (
			SELECT a.id AS application_id,
			ROW_NUMBER() OVER (
				ORDER BY COALESCE(b.priority, 1) DESC, a.id ASC
			) AS new_queue_number
			FROM applications a
			LEFT JOIN benefits b ON a.benefit = b.name
			WHERE a.status NOT IN ('archive', 'rejected') AND a.deleted_at IS NULL
		)
		UPDATE applications
		SET queue_number = Ranked.new_queue_number
		FROM Ranked
		WHERE applications.id = Ranked.application_id;
	`)
	if err != nil {
		fmt.Println("Ошибка пересчёта номеров очереди:", err)
		return
	}

	fmt.Println("Очередь успешно пересчитана")
}

func updateRejectedAndArchived() {
	_, err := DB.Exec(`
		UPDATE applications
		SET queue_number = 0
		WHERE status = 'rejected' OR status = 'archive'
	`)
	if err != nil {
		fmt.Println("Ошибка перевода на 0", err)
		return
	}

	fmt.Println("Успешно переведены на 0")
}
