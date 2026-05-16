-- Seed 20 more users (Total 40)
DO $$
DECLARE
    u_ids UUID[] := '{}';
    u_id UUID;
    i INTEGER;
    pw TEXT := '$argon2id$v=19$m=65536,t=3,p=2$SMAKzRaFETwSZGh5hEk4Ug$9Pv7UTm+8NWb8ANCmY1bcfAS+gRM0ztbYstSNZUQAaA';
    names TEXT[] := ARRAY['Михаил', 'Евгения', 'Николай', 'Дарья', 'Олег', 'Валентина', 'Игорь', 'Марина', 'Андрей', 'Светлана', 'Владимир', 'Инна', 'Григорий', 'Алла', 'Борис', 'Екатерина', 'Александр', 'Надежда', 'Роман', 'Вера'];
    tags TEXT[] := ARRAY['Design', 'Backend', 'Frontend', 'QA', 'Manager', 'Product', 'DevOps', 'Mobile', 'Data'];
BEGIN
    FOR i IN 21..40 LOOP
        u_id := gen_random_uuid();
        u_ids := array_append(u_ids, u_id);
        
        INSERT INTO users (
            id, username, password, role, gender, display_name, tag, telegram_id, promoted_until, promotion_message, promotion_bid, created_at
        ) VALUES (
            u_id,
            'pro_user_' || i,
            pw,
            'user',
            CASE WHEN i % 2 = 0 THEN 'male' ELSE 'female' END,
            names[(i - 21) + 1] || ' ' || i,
            tags[(i % 9) + 1],
            CASE WHEN i % 3 = 0 THEN (2000000 + i)::BIGINT ELSE NULL END,
            CASE WHEN i % 8 = 0 THEN NOW() + interval '1 day' ELSE NULL END,
            CASE WHEN i % 8 = 0 THEN 'VIP User ' || i ELSE NULL END,
            CASE WHEN i % 8 = 0 THEN (i * 2) ELSE 0 END,
            NOW() - interval '2 hours' * i
        );

        INSERT INTO balances (user_id, amount) VALUES (u_id, 500);
    END LOOP;

    -- Generate random hugs for these users to populate speed stats
    FOR i IN 1..20 LOOP
        -- Someone hugs the new user
        INSERT INTO hugs (giver_id, receiver_id, status, hug_type, created_at, accepted_at)
        SELECT id, u_ids[i], 'completed', 'standard', NOW() - interval '10 minutes', NOW() - interval '10 minutes' + (random() * interval '5 minutes')
        FROM users WHERE role = 'admin' LIMIT 1;
        
        -- The new user hugs someone else
        INSERT INTO hugs (giver_id, receiver_id, status, hug_type, created_at, accepted_at)
        SELECT u_ids[i], id, 'completed', 'standard', NOW() - interval '20 minutes', NOW() - interval '20 minutes' + (random() * interval '2 minutes')
        FROM users WHERE username = 'test' LIMIT 1;
    END LOOP;
END $$;
