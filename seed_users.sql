-- Seed Users
DO $$
DECLARE
    u_ids UUID[] := '{}';
    u_id UUID;
    i INTEGER;
    pw TEXT := '$argon2id$v=19$m=65536,t=3,p=2$SMAKzRaFETwSZGh5hEk4Ug$9Pv7UTm+8NWb8ANCmY1bcfAS+gRM0ztbYstSNZUQAaA';
    names TEXT[] := ARRAY['Алексей', 'Мария', 'Дмитрий', 'Елена', 'Иван', 'Анна', 'Сергей', 'Ольга', 'Антон', 'Наталья', 'Виктор', 'Светлана', 'Павел', 'Татьяна', 'Денис', 'Юлия', 'Максим', 'Ирина', 'Артем', 'Ксения'];
    tags TEXT[] := ARRAY['Top-1', 'Люблю обнимашки', 'Быстрый ответ', 'Сплю', 'На работе', 'PROD', 'Junior', 'Senior', 'Босс'];
BEGIN
    FOR i IN 1..20 LOOP
        u_id := gen_random_uuid();
        u_ids := array_append(u_ids, u_id);
        
        INSERT INTO users (
            id, username, password, role, gender, display_name, tag, telegram_id, promoted_until, promotion_message, created_at
        ) VALUES (
            u_id,
            'user_' || i,
            pw,
            'user',
            CASE WHEN i % 2 = 0 THEN 'male' ELSE 'female' END,
            names[(i % 20) + 1] || ' ' || i,
            CASE WHEN i % 3 = 0 THEN tags[(i % 9) + 1] ELSE NULL END,
            CASE WHEN i % 4 = 0 THEN (1000000 + i)::BIGINT ELSE NULL END,
            CASE WHEN i % 7 = 0 THEN NOW() + interval '1 day' * (i % 5 + 1) ELSE NULL END,
            CASE WHEN i % 7 = 0 THEN 'Спонсорское место №' || i ELSE NULL END,
            NOW() - interval '1 day' * i
        );

        INSERT INTO balances (user_id, amount) VALUES (u_id, (i * 10) % 100);
    END LOOP;

    -- Add some hugs to simulate response time
    FOR i IN 1..20 LOOP
        -- Quick user (30s)
        IF i % 5 = 1 THEN
            INSERT INTO hugs (giver_id, receiver_id, status, hug_type, created_at, accepted_at)
            VALUES (u_ids[((i + 1) % 20) + 1], u_ids[i], 'completed', 'standard', NOW() - interval '1 hour', NOW() - interval '1 hour' + interval '30 seconds');
        END IF;

        -- Slow user (2h)
        IF i % 5 = 2 THEN
            INSERT INTO hugs (giver_id, receiver_id, status, hug_type, created_at, accepted_at)
            VALUES (u_ids[((i + 1) % 20) + 1], u_ids[i], 'completed', 'standard', NOW() - interval '5 hour', NOW() - interval '3 hour');
        END IF;
    END LOOP;
END $$;
