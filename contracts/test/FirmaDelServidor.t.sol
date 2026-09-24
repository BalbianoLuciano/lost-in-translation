// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {Base} from "./Base.t.sol";
import {Distinciones} from "../src/Distinciones.sol";

/// @title El test cruzado: Go firma, Solidity verifica.
///
/// @notice Lee `test/fixtures/voucher.json` —que produce un test de Go en
///         `services/api/internal/chain`— y comprueba que el contrato acepta
///         esa firma tal cual vino.
///
/// @dev Es la única prueba de todo el proyecto que puede agarrar un desacuerdo
///      entre las dos implementaciones de EIP-712. Los otros tests del contrato
///      firman con `vm.sign`, o sea que Solidity se prueba contra Solidity: si
///      el servidor ordena distinto los campos del struct, o mete el nombre del
///      dominio crudo en lugar de hasheado, todos ellos siguen en verde y el
///      mint revierte en producción.
///
///      Los dos lados nunca corren juntos. Lo único que comparten es el archivo.
contract FirmaDelServidorTest is Base {
    /// @dev Lo que salió del fixture, guardado para que los tests lo usen.
    uint256 internal chainIdDelFixture;
    address internal deployer;
    uint256 internal nonce;
    address internal contratoEsperado;
    address internal destinatario;
    uint16 internal pieza;
    uint64 internal deadline;
    bytes32 internal digestEsperado;
    bytes internal firmaDeGo;

    /// @dev El problema que resuelve este setUp, y que es lo menos obvio de todo
    ///      el test: el separador de dominio de EIP-712 incluye
    ///      `verifyingContract`, así que Go tuvo que saber la dirección del
    ///      contrato **antes** de que el contrato existiera.
    ///
    ///      La salida es que la dirección de un CREATE no es un azar: sale de
    ///      `keccak256(rlp([deployer, nonce]))`. Con un deployer fijo y un nonce
    ///      fijo, los dos lados llegan al mismo número sin hablarse: Go lo
    ///      calcula, acá se reproduce con `vm.setNonce` + `vm.prank`.
    ///
    ///      Y se afirma. Si alguna vez no coincide, el test tiene que fallar
    ///      acá, con un mensaje que diga exactamente eso, y no doscientas líneas
    ///      más abajo con un `FirmaInvalida` que no explica nada.
    function setUp() public override {
        string memory json = vm.readFile("test/fixtures/voucher.json");

        chainIdDelFixture = vm.parseJsonUint(json, ".chainId");
        deployer = vm.parseJsonAddress(json, ".deployer");
        nonce = vm.parseJsonUint(json, ".nonce");
        contratoEsperado = vm.parseJsonAddress(json, ".verifyingContract");
        firmante = vm.parseJsonAddress(json, ".firmante");
        destinatario = vm.parseJsonAddress(json, ".to");
        pieza = uint16(vm.parseJsonUint(json, ".pieza"));
        deadline = uint64(vm.parseJsonUint(json, ".deadline"));
        digestEsperado = vm.parseJsonBytes32(json, ".digest");
        firmaDeGo = vm.parseJsonBytes(json, ".firma");

        // La cadena, antes de desplegar: OpenZeppelin cachea el separador de
        // dominio en el constructor, y queremos que lo cachee con este chainId.
        vm.chainId(chainIdDelFixture);

        vm.setNonce(deployer, uint64(nonce));
        vm.prank(deployer);
        distinciones = new Distinciones(duenio, firmante, _catalogo());

        assertEq(
            address(distinciones),
            contratoEsperado,
            unicode"la dirección del contrato no es la que usó Go para firmar: revisá el deployer y el nonce del fixture antes de mirar la firma"
        );

        // El voucher vence en 2033; el reloj del test arranca en 1. Igual se
        // adelanta hasta un rato antes del vencimiento, para que la prueba no
        // dependa de estar corriendo en el año cero de la EVM.
        vm.warp(deadline - 10 minutes);
    }

    /// @notice El fixture y el contrato calculan el mismo digest.
    /// @dev Se chequea por separado del mint porque separa dos fallas que se
    ///      ven igual desde afuera: "el hash no coincide" y "la firma no es del
    ///      firmante". Con esto, cuando el mint falle, ya sabemos cuál de las
    ///      dos es.
    function test_ElDigestDelFixtureEsElQueCalculaElContrato() public view {
        assertEq(
            _digest(destinatario, pieza, deadline),
            digestEsperado,
            unicode"el digest de EIP-712 no coincide: cambió el tipo del struct, el orden de un campo o el separador de dominio"
        );
    }

    /// @notice El contrato acuña con la firma que hizo Go. Esto es la etapa C.3.
    function test_ElContratoAceptaLaFirmaDeGo() public {
        assertEq(distinciones.firmante(), firmante, "el firmante del fixture no quedo configurado");

        distinciones.mint(destinatario, pieza, deadline, firmaDeGo);

        uint256 id = distinciones.idDe(destinatario, pieza);
        assertEq(distinciones.ownerOf(id), destinatario, "la distincion no quedo a nombre de `to`");
        assertTrue(distinciones.locked(id), "la distincion tendria que estar trabada");
    }

    /// @notice La firma de Go tiene la S baja, que es la única que OpenZeppelin
    ///         acepta.
    /// @dev Una firma con la S alta es matemáticamente válida y `tryRecover` la
    ///      rechaza igual, por maleable. Si el servidor no normalizara, la mitad
    ///      de los vouchers fallaría sin ningún patrón visible: el tipo de bug
    ///      que se busca durante días.
    function test_LaFirmaVieneNormalizada() public view {
        assertEq(firmaDeGo.length, 65, "la firma tiene que ser R||S||V");

        uint8 v = uint8(firmaDeGo[64]);
        assertTrue(v == 27 || v == 28, "V tiene que ser 27 o 28");

        // La copia en memoria no es un rodeo: el ensamblador no puede leer una
        // variable de storage con `mload`.
        bytes memory firma = firmaDeGo;

        bytes32 s;
        // Un `bytes` en memoria arranca con 32 bytes de largo, así que +32 cae
        // en R y +64, en S.
        assembly {
            s := mload(add(firma, 64))
        }
        assertLe(
            uint256(s), ORDEN_SECP256K1 / 2, "la S esta en la mitad alta: la firma es maleable"
        );
    }

    /// @notice Cambiarle un solo campo al voucher tira abajo la firma.
    /// @dev Es la propiedad que hace que el voucher sirva de algo: si alguien
    ///      pudiera mover la pieza, el deadline o el destinatario y la firma
    ///      siguiera valiendo, el servidor no estaría autorizando nada.
    function test_UnSoloCampoCambiadoInvalidaLaFirma() public {
        address otro = makeAddr("otro");

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(otro, pieza, deadline, firmaDeGo);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(destinatario, pieza + 1, deadline, firmaDeGo);

        // Un segundo más de vida: cambia el hash igual que cualquier otra cosa.
        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(destinatario, pieza, deadline + 1, firmaDeGo);
    }

    /// @notice Tocarle un bit a la firma también la tira abajo.
    /// @dev Un bit en R o en S da otra firma, de otra clave cualquiera; un bit
    ///      en V hace que `ecrecover` recupere el otro punto de la curva. En los
    ///      tres casos el contrato ve a alguien que no es el firmante.
    function test_UnBitCambiadoEnLaFirmaLaInvalida() public {
        for (uint256 i = 0; i < 3; ++i) {
            // Un byte de R, uno de S y el de V.
            uint256 posicion = [uint256(0), 40, 64][i];

            bytes memory rota = bytes.concat(firmaDeGo);
            rota[posicion] = bytes1(uint8(rota[posicion]) ^ 0x01);

            // No se afirma el error puntual: con V dado vuelta el contrato
            // revierte con FirmaInvalida, y lo que importa es que en ningún caso
            // acuñe.
            vm.expectRevert();
            distinciones.mint(destinatario, pieza, deadline, rota);
        }
    }

    /// @notice El voucher vencido no sirve, aunque la firma esté perfecta.
    function test_LaFirmaDeGoNoSirveDespuesDelDeadline() public {
        vm.warp(uint256(deadline) + 1);

        vm.expectRevert(Distinciones.Vencido.selector);
        distinciones.mint(destinatario, pieza, deadline, firmaDeGo);
    }

    /// @notice La firma no sirve en otra cadena.
    /// @dev El dominio incluye el chainId, así que una firma de la testnet no
    ///      vale en mainnet. Acá se ve directo: al mover el chainId, el contrato
    ///      recalcula su separador —el caché de OpenZeppelin justamente vence
    ///      cuando la cadena cambia— y la misma firma deja de servir.
    function test_LaFirmaDeGoNoSirveEnOtraCadena() public {
        vm.chainId(chainIdDelFixture + 1);

        vm.expectRevert(Distinciones.FirmaInvalida.selector);
        distinciones.mint(destinatario, pieza, deadline, firmaDeGo);
    }
}
