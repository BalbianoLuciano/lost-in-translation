-- +goose Up
-- El óxido necesita memoria. Con el promedio pesado de los últimos intentos, una
-- pieza calzada nunca cae de golpe: pasa por suspendida antes de cruzar el piso,
-- así que mirar sólo el estado anterior la mandaba a plano y no se oxidaba nunca.
-- Esta columna recuerda el hecho —la pieza estuvo calzada— y no se borra: es lo
-- que distingue una pieza que se aflojó de una que nunca se levantó.
--
-- No sale de `achievements` a propósito: el logro es la historia de la persona,
-- el dominio es el estado del tema. Atar un paquete al otro los junta sin motivo.
ALTER TABLE skill_mastery ADD COLUMN was_calzada boolean NOT NULL DEFAULT false;

-- Lo que hoy está calzado ya estuvo calzado, y lo que hoy está oxidado también
-- (para oxidarse hay que haber calzado antes). Sin este UPDATE el óxido
-- empezaría a contar recién desde la próxima vez que cada tema quede firme, y
-- las piezas que ya están oxidadas perderían el óxido en el primer recálculo.
UPDATE skill_mastery SET was_calzada = true WHERE state IN ('calzada', 'oxidada');

-- +goose Down
ALTER TABLE skill_mastery DROP COLUMN was_calzada;
