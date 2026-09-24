// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {ERC721} from "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {EIP712} from "@openzeppelin/contracts/utils/cryptography/EIP712.sol";
import {ECDSA} from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import {Base64} from "@openzeppelin/contracts/utils/Base64.sol";
import {Strings} from "@openzeppelin/contracts/utils/Strings.sol";

import {IERC5192} from "./IERC5192.sol";
import {Pilar} from "./Pilar.sol";

/// @title Distinciones — el recibo, en la cadena, de algo que costó aprender.
/// @notice Un ERC-721 que no se transfiere. Cada token dice que alguien, alguna
///         vez, calzó una pieza o terminó una obra en Lost in Translation.
///
/// @dev La cadena es el recibo, no la verdad: el progreso vive en Postgres y el
///      servidor decide quién se ganó qué. Este contrato sólo verifica que el
///      servidor lo haya firmado, y no guarda un solo dato de la persona: una
///      dirección y un número de pieza, nada más.
contract Distinciones is ERC721, IERC5192, EIP712, Ownable {
    /// @notice Una de las 41 distinciones posibles: 34 piezas y 7 obras.
    /// @dev Se cargan enteras al desplegar. No hay forma de agregar ni sacar:
    ///      el catálogo sale de `content/skills.yaml` y es fijo por diseño.
    ///
    ///      Los dos índices de junta son lo que hace que la pieza sea *esta* y
    ///      no otra: la junta de abajo de una pieza tiene que ser la de arriba
    ///      de la siguiente de su obra, porque es la misma línea de los dos
    ///      lados. Eso lo garantiza quien arma el catálogo, no el contrato: acá
    ///      sólo se chequea que el perfil exista.
    struct Pieza {
        string nombre;
        uint8 obra;
        uint8 juntaArriba;
        uint8 juntaAbajo;
        /// @dev Falso: la distinción dibuja esta pieza sola. Verdadero: dibuja
        ///      el pilar entero de su obra. Son las 7 últimas del catálogo.
        bool esObra;
    }

    /// @dev El tipo del voucher. Si cambia el orden o el nombre de un campo,
    ///      cambia el hash, y las firmas viejas dejan de servir. Es a propósito.
    bytes32 private constant TIPO_DISTINCION =
        keccak256("Distincion(address to,uint16 pieza,uint64 deadline)");

    /// @notice La clave que firma los vouchers. No tiene fondos: no puede gastar.
    /// @dev Se puede rotar con `setFirmante` sin volver a desplegar, que es toda
    ///      la respuesta que tenemos si el secreto del servidor se filtra.
    address public firmante;

    /// @notice El catálogo, en el mismo orden que el número de pieza del voucher.
    Pieza[] public piezas;

    /// @notice La firma no la hizo el firmante, o no se pudo recuperar nada de ella.
    error FirmaInvalida();
    /// @notice El voucher tiene fecha de vencimiento corta, justamente para esto.
    error Vencido();
    /// @notice Se intentó mover un token. No se mueven.
    error NoSeTransfiere();
    /// @notice El número de pieza no está en el catálogo.
    error PiezaInexistente(uint16 pieza);
    /// @notice Esa distinción ya está acuñada. Un voucher no se usa dos veces.
    error YaReclamada(uint256 tokenId);
    /// @notice Un firmante en cero dejaría el contrato acuñando para cualquiera.
    error FirmanteVacio();
    /// @notice Desplegar sin catálogo dejaría un contrato que no puede acuñar nada.
    error SinPiezas();
    /// @notice El catálogo pide un perfil de junta que no existe. Hay diez.
    error JuntaInexistente(uint256 indice, uint8 junta);

    /// @notice Queda en el registro de eventos quién firmaba antes y quién ahora.
    event FirmanteCambiado(address indexed anterior, address indexed nuevo);

    /// @param duenio Quien puede rotar el firmante.
    /// @param firmanteInicial La dirección de la clave de firma del servidor.
    /// @param catalogo Las 41 distinciones. Llegan por parámetro y no escritas
    ///        acá adentro porque el catálogo es contenido, no código.
    constructor(address duenio, address firmanteInicial, Pieza[] memory catalogo)
        ERC721("Distinciones", "DIST")
        EIP712("Lost in Translation", "1")
        Ownable(duenio)
    {
        if (firmanteInicial == address(0)) revert FirmanteVacio();
        if (catalogo.length == 0) revert SinPiezas();

        firmante = firmanteInicial;
        emit FirmanteCambiado(address(0), firmanteInicial);

        for (uint256 i; i < catalogo.length; ++i) {
            // El catálogo entra una sola vez y no se puede corregir. Un índice
            // de junta fuera de rango dejaría un `tokenURI` que revierte para
            // siempre, así que se chequea acá, donde todavía se puede abortar.
            // Revertir adentro del lazo es justo lo que se quiere: un catálogo
            // con una junta inválida no se despliega a medias.
            if (catalogo[i].juntaArriba >= Pilar.JUNTAS) {
                // forge-lint: disable-next-line(require-revert-in-loop)
                revert JuntaInexistente(i, catalogo[i].juntaArriba);
            }
            if (catalogo[i].juntaAbajo >= Pilar.JUNTAS) {
                // forge-lint: disable-next-line(require-revert-in-loop)
                revert JuntaInexistente(i, catalogo[i].juntaAbajo);
            }
            piezas.push(catalogo[i]);
        }
    }

    // ---------------------------------------------------------------- acuñar

    /// @notice Acuña la distinción `pieza` a nombre de `to`, si el servidor lo firmó.
    /// @dev Cualquiera puede mandar esta transacción, incluso alguien que no sea
    ///      `to`: el voucher ata el destinatario, así que adelantarse en el
    ///      mempool sólo consigue pagarle el gas a otro.
    function mint(address to, uint16 pieza, uint64 deadline, bytes calldata firma) external {
        // El reloj del bloque lo puede correr un validador unos segundos. Da
        // igual: el deadline se mide en minutos y sólo acota la ventana de una
        // firma que ya es válida.
        // forge-lint: disable-next-line(block-timestamp)
        if (block.timestamp > deadline) revert Vencido();
        if (pieza >= piezas.length) revert PiezaInexistente(pieza);

        bytes32 digest =
            _hashTypedDataV4(keccak256(abi.encode(TIPO_DISTINCION, to, pieza, deadline)));

        // tryRecover en lugar de recover: devuelve el error en vez de revertir con
        // el suyo, y de paso rechaza las firmas maleables (la mitad alta de `s`).
        (address recuperado, ECDSA.RecoverError error_,) = ECDSA.tryRecover(digest, firma);
        if (error_ != ECDSA.RecoverError.NoError || recuperado != firmante) {
            revert FirmaInvalida();
        }

        // Reclamar dos veces la misma pieza choca con un id que ya existe: no
        // hace falta ninguna tabla de reclamadas, alcanza con mirar el dueño.
        // El `_mint` de OpenZeppelin también lo rechazaría, pero lo haría después
        // de pasar por `_update`, y ahí el error que sale es "no se transfiere",
        // que para quien reclama dos veces no explica nada.
        uint256 id = idDe(to, pieza);
        if (_ownerOf(id) != address(0)) revert YaReclamada(id);
        // `_mint` y no `_safeMint`: el aviso al receptor existe para que un
        // contrato no se quede con un token que no sabe mover, y acá nadie puede
        // mover nada. A cambio, esta función no llama a ningún contrato ajeno.
        // forge-lint: disable-next-line(unsafe-oz-erc721-mint)
        _mint(to, id);
        // `_mint`, a diferencia de `_safeMint`, no llama a nadie: no hay reentrada
        // posible entre estas dos líneas.
        // forge-lint: disable-next-line(reentrancy-events)
        emit Locked(id);
    }

    /// @notice El id de la distinción `pieza` de `titular`, sin consultar nada.
    /// @dev Determinista a propósito: el front puede mostrar el token antes de
    ///      que exista, y la pieza se lee del id con `uint16(tokenId)`.
    function idDe(address titular, uint16 pieza) public pure returns (uint256) {
        return (uint256(uint160(titular)) << 16) | pieza;
    }

    /// @notice Cuántas distinciones tiene el catálogo.
    function cantidadPiezas() external view returns (uint256) {
        return piezas.length;
    }

    // ------------------------------------------------------------- soulbound

    /// @notice Todas las distinciones están trabadas, siempre.
    /// @dev El estándar pide que esto reverta para un token que no existe, y por
    ///      eso no es `pure`: preguntar por el candado de algo inexistente es una
    ///      pregunta mal hecha, no un "sí".
    function locked(uint256 tokenId) external view returns (bool) {
        _requireOwned(tokenId);
        return true;
    }

    /// @dev Toda la prohibición de mover vive acá: `_update` es por donde pasan
    ///      `transferFrom`, `safeTransferFrom`, el minteo y el quemado. Dejamos
    ///      abierto sólo el minteo (`from == 0`).
    function _update(address to, uint256 tokenId, address auth)
        internal
        override
        returns (address)
    {
        address from = _ownerOf(tokenId);
        if (from != address(0)) revert NoSeTransfiere();
        return super._update(to, tokenId, auth);
    }

    // ------------------------------------------------------------- metadatos

    /// @notice Los metadatos del token, enteros adentro del propio contrato:
    ///         el JSON y el SVG salen de acá, no de un servidor que algún día
    ///         se apaga.
    ///
    /// @dev Es `view`, así que leerla no cuesta gas: el dibujo se arma entero en
    ///      el nodo que responde la consulta. Por eso el contrato puede darse el
    ///      lujo de concatenar cientos de strings sin que le importe a nadie.
    function tokenURI(uint256 tokenId) public view override returns (string memory) {
        _requireOwned(tokenId);

        // La truncada es el punto: los 16 bits de abajo del id son la pieza, y
        // `_requireOwned` ya garantizó que el token existe, así que el índice
        // estaba dentro del catálogo cuando se acuñó.
        // forge-lint: disable-next-line(unsafe-typecast)
        Pieza storage p = piezas[uint16(tokenId)];

        // El nombre sale del catálogo, que lo puso quien desplegó; igual se
        // escapa, porque el mismo texto entra a un JSON y a un XML y una
        // comilla suelta rompe los dos.
        string memory nombre = _escapar(p.nombre);
        (uint8[] memory arriba, uint8[] memory abajo) = _juntasDelDibujo(p);

        string memory imagen = string.concat(
            "data:image/svg+xml;base64,", Base64.encode(bytes(Pilar.svg(nombre, arriba, abajo)))
        );

        string memory json = string.concat(
            '{"name":"',
            nombre,
            '","description":"Distincion de Lost in Translation. Obra ',
            Strings.toString(p.obra),
            '. No se transfiere: es de quien la gano.","attributes":[{"trait_type":"Clase","value":"',
            p.esObra ? "obra" : "pieza",
            '"},{"trait_type":"Obra","value":',
            Strings.toString(p.obra),
            '}],"image":"',
            imagen,
            '"}'
        );

        return string.concat("data:application/json;base64,", Base64.encode(bytes(json)));
    }

    /// @notice El SVG solo, sin el JSON ni el base64 encima.
    /// @dev No lo pide ningún estándar; existe porque mirar el dibujo mientras
    ///      se lo escribe, sin desarmar dos capas de base64, vale su peso.
    function svgDe(uint256 tokenId) external view returns (string memory) {
        _requireOwned(tokenId);
        // forge-lint: disable-next-line(unsafe-typecast)
        Pieza storage p = piezas[uint16(tokenId)];
        (uint8[] memory arriba, uint8[] memory abajo) = _juntasDelDibujo(p);
        return Pilar.svg(_escapar(p.nombre), arriba, abajo);
    }

    /// @dev Qué piezas dibuja una distinción: una sola si es de pieza, todas las
    ///      de su obra si es de obra. Las de la obra salen en el orden del
    ///      catálogo, que es el orden en que se apilan, y por eso la junta de
    ///      abajo de cada una es la de arriba de la que sigue.
    function _juntasDelDibujo(Pieza storage p)
        private
        view
        returns (uint8[] memory arriba, uint8[] memory abajo)
    {
        if (!p.esObra) {
            arriba = new uint8[](1);
            abajo = new uint8[](1);
            arriba[0] = p.juntaArriba;
            abajo[0] = p.juntaAbajo;
            return (arriba, abajo);
        }

        uint8 obra = p.obra;
        uint256 total = piezas.length;

        uint256 cuantas = 0;
        for (uint256 i; i < total; ++i) {
            if (!piezas[i].esObra && piezas[i].obra == obra) ++cuantas;
        }

        // Una obra a la que todavía no se le escribió el contenido no tiene
        // piezas en el catálogo. Antes que revertir en una función que las
        // billeteras llaman solas, dibuja una piedra sola, de juntas rectas: es
        // exactamente lo que hay.
        if (cuantas == 0) cuantas = 1;

        arriba = new uint8[](cuantas);
        abajo = new uint8[](cuantas);

        uint256 k = 0;
        for (uint256 i; i < total; ++i) {
            Pieza storage q = piezas[i];
            if (q.esObra || q.obra != obra) continue;
            arriba[k] = q.juntaArriba;
            abajo[k] = q.juntaAbajo;
            ++k;
        }
    }

    /// @dev Los cinco caracteres que rompen un JSON o un XML, cambiados por sus
    ///      entidades. `&#92;` es la contrabarra: en el SVG se ve como tal y en
    ///      el JSON queda como texto, que es todo lo que hace falta.
    ///
    ///      El camino rápido es no tocar nada: ningún nombre del catálogo tiene
    ///      estos caracteres, así que en la práctica esto recorre el nombre una
    ///      vez y devuelve el mismo string.
    function _escapar(string memory s) private pure returns (string memory) {
        bytes memory b = bytes(s);
        bool hace = false;
        for (uint256 i; i < b.length; ++i) {
            if (_rompe(b[i])) {
                hace = true;
                break;
            }
        }
        if (!hace) return s;

        string memory fuera;
        for (uint256 i; i < b.length; ++i) {
            bytes1 c = b[i];
            if (c == "&") fuera = string.concat(fuera, "&amp;");
            else if (c == "<") fuera = string.concat(fuera, "&lt;");
            else if (c == ">") fuera = string.concat(fuera, "&gt;");
            else if (c == '"') fuera = string.concat(fuera, "&quot;");
            else if (c == "\\") fuera = string.concat(fuera, "&#92;");
            else fuera = string.concat(fuera, string(abi.encodePacked(c)));
        }
        return fuera;
    }

    function _rompe(bytes1 c) private pure returns (bool) {
        return c == "&" || c == "<" || c == ">" || c == '"' || c == "\\";
    }

    /// @dev ERC-721, ERC-721 Metadata y ERC-165 los trae OpenZeppelin; el 5192 es
    ///      el que hace que una billetera pueda saber que esto no se mueve.
    function supportsInterface(bytes4 interfaceId) public view override returns (bool) {
        return interfaceId == type(IERC5192).interfaceId || super.supportsInterface(interfaceId);
    }

    // ---------------------------------------------------------------- rotación

    /// @notice Cambia la clave que firma los vouchers.
    /// @dev Es la única palanca de administración del contrato, y existe para un
    ///      solo caso: que se filtre el secreto del servidor. No puede acuñar, no
    ///      puede mover tokens y no puede tocar el catálogo.
    function setFirmante(address nuevo) external onlyOwner {
        if (nuevo == address(0)) revert FirmanteVacio();
        emit FirmanteCambiado(firmante, nuevo);
        firmante = nuevo;
    }
}
