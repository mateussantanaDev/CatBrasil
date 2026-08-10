const jwt = require('jsonwebtoken');
const JWT_SECRET = process.env.JWT_SECRET || 'super_secret_cat_token_987654321_key';

function authenticateToken(req, res, next) {
    const authHeader = req.headers['authorization'];
    const token = authHeader && authHeader.split(' ')[1];

    if (!token) {
        return res.status(401).json({ error: "Acesso não autorizado. Token ausente." });
    }

    jwt.verify(token, JWT_SECRET, (err, decoded) => {
        if (err) {
            return res.status(403).json({ error: "Sessão expirada ou token inválido." });
        }
        // Armazena o userId descriptografado no request
        req.userId = decoded.userId;
        next();
    });
}

module.exports = authenticateToken;
